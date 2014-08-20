// Copyright 2014 Apcera Inc. All rights reserved.

package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const (
	dockerDefaultRepoPrefix = "library"
)

var (
	dockerIndexURL = "https://index.docker.io" // Tests can change it to point to a mock registry.
)

// Image is a Docker image info (constructed from Docker API response).
type Image struct {
	Name string

	tags      map[string]string // Tags available for the image.
	endpoints []string          // Docker registry endpoints.
	token     string            // Docker auth token.

	// scheme is an original index URL scheme (will be used to talk to endpoints returned by API).
	scheme string
	client *http.Client
}

// GetImage fetches Docker repository information from Docker index.
func GetImage(name string) (*Image, error) {
	if name == "" {
		return nil, errors.New("image name is empty")
	}

	if strings.Count(name, "/") == 0 {
		name = fmt.Sprintf("%s/%s", dockerDefaultRepoPrefix, name)
	}

	u, err := url.Parse(dockerIndexURL)
	if err != nil {
		return nil, err
	}

	// In order to get layers from Docker CDN we need to hit 'images' endpoint
	// and request the token. Client should also accept and store cookies, as
	// they are needed to fetch the layer data later.
	imagesURL := fmt.Sprintf("%s/v1/repositories/%s/images", dockerIndexURL, name)

	req, err := http.NewRequest("GET", imagesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Docker-Token", "true")

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
	client.Jar, err = cookiejar.New(nil) // Docker repo API sets and uses cookies for CDN.
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d ", res.StatusCode)
	}

	token := res.Header.Get("X-Docker-Token")
	endpoints := strings.Split(res.Header.Get("X-Docker-Endpoints"), ",")

	if len(endpoints) == 0 {
		return nil, errors.New("Docker index response didn't contain any endpoints")
	}
	for i := range endpoints {
		endpoints[i] = strings.Trim(endpoints[i], " ")
	}

	img := &Image{
		Name:      name,
		client:    client,
		endpoints: endpoints,
		token:     token,
		scheme:    u.Scheme,
	}

	img.tags, err = img.fetchTags()
	if err != nil {
		return nil, err
	}

	return img, nil
}

// TagLayerID returns a layer ID for a given tag.
func (i *Image) TagLayerID(tagName string) (string, error) {
	layerID, ok := i.tags[tagName]
	if !ok {
		return "", fmt.Errorf("can't find tag '%s' for image '%s'", tagName, i.Name)
	}

	return layerID, nil
}

// Metadata unmarshals a Docker image metadata into provided 'v' interface.
func (i *Image) Metadata(tagName string, v interface{}) error {
	layerID, ok := i.tags[tagName]
	if !ok {
		return fmt.Errorf("can't find tag '%s' for image '%s'", tagName, i.Name)
	}

	err := i.parseResponse(fmt.Sprintf("v1/images/%s/json", layerID), &v)
	if err != nil {
		return err
	}
	return nil
}

// History returns an ordered list of layers that make up Docker. The order is reverse, it goes from
// the latest layer to the base layer. Client can iterate these layers and download them using LayerReader.
func (i *Image) History(tagName string) ([]string, error) {
	layerID, ok := i.tags[tagName]
	if !ok {
		return nil, fmt.Errorf("can't find tag '%s' for image '%s'", tagName, i.Name)
	}

	var history []string
	err := i.parseResponse(fmt.Sprintf("v1/images/%s/ancestry", layerID), &history)
	if err != nil {
		return nil, err
	}
	return history, nil
}

// LayerReader returns io.ReadCloser that can be used to read Docker layer data.
func (i *Image) LayerReader(id string) (io.ReadCloser, error) {
	resp, err := i.getResponse(fmt.Sprintf("v1/images/%s/layer", id))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// fetchTags fetches tags for the image and caches them in the Image struct,
// so that other methods can look them up efficiently.
func (i *Image) fetchTags() (map[string]string, error) {
	// There is a weird quirk about Docker API: if tags are requested from index.docker.io,
	// it returns a list of short layer IDs, so it's impossible to use them to download actual layers.
	// However, when we hit the endpoint returned by image index API response, it has an expected format.
	var tags map[string]string
	err := i.parseResponse(fmt.Sprintf("v1/repositories/%s/tags", i.Name), &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// getAPIResponse takes a path and tries to get Docker API response from each
// available Docker API endpoint. It returns raw HTTP response.
func (i *Image) getResponse(path string) (*http.Response, error) {
	errors := make(map[string]error)

	for _, ep := range i.endpoints {
		resp, err := i.getResponseFromURL(fmt.Sprintf("%s://%s/%s", i.scheme, ep, path))
		if err != nil {
			errors[ep] = err
			continue
		}

		return resp, nil
	}

	return nil, combineEndpointErrors(errors)
}

// parseJSONResponse takes a path and tries to get Docker API response from each
// available Docker API endpoint. It tries to parse response as JSON and saves
// the parsed version in the provided 'result' variable.
func (i *Image) parseResponse(path string, result interface{}) error {
	errors := make(map[string]error)

	for _, ep := range i.endpoints {
		err := i.parseResponseFromURL(fmt.Sprintf("%s://%s/%s", i.scheme, ep, path), result)
		if err != nil {
			errors[ep] = err
			continue
		}

		return nil
	}

	return combineEndpointErrors(errors)
}

// getAPIResponseFromURL returns raw Docker API response at URL 'u'.
func (i *Image) getResponseFromURL(u string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+i.token)

	res, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("%s: HTTP %d", u, res.StatusCode)
	}

	return res, nil
}

// parseResponseFromURL returns parsed JSON of a Docker API response at URL 'u'.
func (i *Image) parseResponseFromURL(u string, result interface{}) error {
	resp, err := i.getResponseFromURL(u)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return err
	}

	return nil
}

// combineEndpointErrors takes a mapping of Docker API endpoints to errors encountered
// while talking to them and returns a single error that contains all endpoint URLs
// along with error for each URL.
func combineEndpointErrors(allErrors map[string]error) error {
	var parts []string
	for ep, err := range allErrors {
		parts = append(parts, fmt.Sprintf("%s: %s", ep, err))
	}
	return errors.New(strings.Join(parts, ", "))
}
