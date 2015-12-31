// Copyright 2015 Apcera Inc. All rights reserved.

// v2 is a Docker v2 Registry API client implementation. Currently only supports
// authenticating, pulling images, and pulling layers.
//
// See: https://docs.docker.com/registry/spec/api/
package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"

	"github.com/apcera/util/docker"
	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/digest"
	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/manifest/schema1"
	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/client/auth"
)

var (
	// DockerHubRegistryURL points to the v2 registry URL.
	DockerHubRegistryURL = "https://registry-1.docker.io"

	// ErrUnsupported is an error returned if the target registry does not
	// support the v2 API.
	ErrUnsupported = errors.New("Registry does not support v2 API")
)

// A DockerClient pulls a single Docker image from a registry that conforms to
// the v2 Docker registry API.
type DockerClient struct {
	imageURL        *docker.DockerRegistryURL
	authChallenge   auth.Challenge
	imageFetchToken string
	imageMetadata   map[string]interface{}
	httpClient      *http.Client
}

// NewDockerClient initializes a new client for fetching Docker images and
// layers. It should be initialized with a fully qualified image URL including
// tag/reference and any username/password.
func NewDockerClient(imageURL *docker.DockerRegistryURL) (*DockerClient, error) {
	client := &DockerClient{
		imageURL:   imageURL,
		httpClient: &http.Client{},
	}

	var err error
	client.httpClient.Jar, err = cookiejar.New(nil) // Private registries (like Quay) like cookies
	if err != nil {
		return nil, err
	}

	return client, nil
}

// NewDockerClientFromRawURL is a convenience wrapper for initalizing a
// DockerClient from a raw string URL rather than a typed URL.
func NewDockerClientFromRawURL(rawURL string) (*DockerClient, error) {
	imageURL, err := docker.ParseDockerRegistryURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Docker image URL: %s", err)
	}

	return NewDockerClient(imageURL)
}

// CheckV2Support checks to see if a registry supports the v2 API. If not, an
// error is returned.
func (d *DockerClient) CheckV2Support() error {
	u, err := url.Parse(v2SupportCheckURL(d.imageURL))
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Get(u.String())
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		token, err := d.authenticate(resp)
		if err != nil {
			return err
		}
		d.imageFetchToken = token
	default:
		return ErrUnsupported
	}
	return nil
}

func v2SupportCheckURL(imageURL *docker.DockerRegistryURL) string {
	return fmt.Sprintf("%s://%s", imageURL.Scheme, path.Join(imageURL.HostPort(), "v2"))
}

// FetchImage fetches a new image from a Docker v2 API-compatible registry. The
// public Docker Hub implements this API exclusively, as do any new registries.
//
// See: https://docs.docker.com/registr/yspec/api/
func (d *DockerClient) FetchImage() ([]string, error) {
	unparsedImageURL := imageManifestURL(d.imageURL)

	imageURL, err := url.Parse(unparsedImageURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", imageURL.String(), nil)
	if err != nil {
		return nil, err
	}

	for _, cookie := range d.httpClient.Jar.Cookies(imageURL) {
		req.AddCookie(cookie)
	}

	if d.imageFetchToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", d.imageFetchToken))
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// OK
	case http.StatusUnauthorized:
		// Should have fetched a token during the v2 check; this may indicate
		// that the image wasn't found, as the Docker registry would rather
		// return a 401 than a 404.
		// TODO: consider inspecting the 'error' field from the Challenge header
		// to see if it's useful to an end user.
		return nil, fmt.Errorf("not authorized to pull %q", d.imageURL.StringNoCredentials())
	default:
		return nil, fmt.Errorf("bad response from registry: %q", resp.Status)
	}

	// TODO: add signature verification support here by using SignedManifest
	// type w/ libtrust; maybe make configurable.
	var imageManifest schema1.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&imageManifest); err != nil {
		return nil, err
	}
	resp.Body.Close()

	var meta map[string]interface{}

	// We don't merge Docker container config metadata from across multiple
	// layers; instead, we respect the latest layer's container config, and we
	// ignore the rest.
	// TODO: do better than just take latest layer?
	if len(imageManifest.History) > 0 && len(imageManifest.History[0].V1Compatibility) > 0 {
		if err := json.Unmarshal([]byte(imageManifest.History[0].V1Compatibility), &meta); err != nil {
			return nil, err
		}
	}
	d.imageMetadata = meta

	var layerIDs []string
	for _, fsLayer := range imageManifest.FSLayers {
		layerIDs = append(layerIDs, fsLayer.BlobSum.String())
	}

	return layerIDs, nil
}

func imageManifestURL(imageURL *docker.DockerRegistryURL) string {
	return fmt.Sprintf("%s://%s", imageURL.Scheme, path.Join(imageURL.HostPort(),
		"v2", imageURL.ImageName, "manifests", imageURL.Tag))
}

// LayerReader returns an io.ReadCloser that can be used to read Docker layer
// data. Caller is responsible for closing the ReadCloser.
func (d *DockerClient) LayerReader(layerID string) (io.ReadCloser, error) {
	layerDigest, err := digest.ParseDigest(layerID)
	if err != nil {
		return nil, err
	}

	token, err := d.fetchToken(d.authChallenge)
	if err != nil {
		return nil, err
	}

	layerURL, err := url.Parse(layerDownloadURL(d.imageURL, layerDigest))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", layerURL.String(), nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	for _, cookie := range d.httpClient.Jar.Cookies(layerURL) {
		req.AddCookie(cookie)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// OK
	case http.StatusUnauthorized:
		// TODO: consider inspecting the 'error' field from the Challenge header
		// to see if it's useful to an end user.
		return nil, fmt.Errorf("not authorized to pull layer %q", layerDigest)
	default:
		return nil, fmt.Errorf("failed to fetch layer: %s", resp.Status)
	}

	return resp.Body, nil
}

func layerDownloadURL(imageURL *docker.DockerRegistryURL, blobSum digest.Digest) string {
	return fmt.Sprintf("%s://%s/v2/%s/blobs/%s",
		imageURL.Scheme, imageURL.HostPort(), imageURL.ImageName, blobSum.String())
}

// RawMetadata returns the image metadata.
func (d *DockerClient) RawMetadata() map[string]interface{} {
	return d.imageMetadata
}
