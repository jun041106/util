// Copyright 2015 Apcera Inc. All rights reserved.

// v2 is a Docker v2 Registry API client implementation.
//
// See: https://docs.docker.com/registry/spec/api/
package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	// TODO: godep?
	"github.com/docker/distribution/registry/client/auth"
)

// authenticate retrieves an authentication token for an action against a V2
// registry. Actions are scoped to certain repositories, images, and actions.
// Tokens also are typically one-time use; i.e., one token per layer.
//
// V2 registries that require authentication will supply a WWW-Authenticate
// header on a 401 response with the below as an example:
// Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nats"
//
// See: https://docs.docker.com/registry/spec/auth/token/
func (d *DockerClient) authenticate(resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("expected HTTP Status %d on challenge; got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	challenges := auth.ResponseChallenges(resp)

	// TODO: add support for handling multiple auth challenges?
	if len(challenges) != 1 {
		return "", fmt.Errorf("got %d authentication challenges", len(challenges))
	}

	token, err := d.fetchToken(challenges[0])
	if err != nil {
		return "", err
	}

	// Store a copy of the original challenge. As we need one token per request,
	// we'll need to request more tokens with the same scope.
	d.authChallenge = challenges[0]

	return token, nil
}

// fetchToken handles an authentication challenge and returns a token that
// allows us to access a registry.
func (d *DockerClient) fetchToken(challenge auth.Challenge) (string, error) {
	if challenge.Parameters == nil {
		return "", errors.New("must provide challenge for token fetch")
	}

	rawURL, err := buildAuthenticationURL(challenge.Parameters, d.imageURL.ImageName)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	if d.imageURL.Userinfo != "" {
		authParts := strings.SplitN(d.imageURL.Userinfo, ":", 2)
		if len(authParts) != 2 {
			return "", errors.New("malformed basic authentication credentials")
		}
		req.SetBasicAuth(authParts[0], authParts[1])
	}

	for _, cookie := range d.httpClient.Jar.Cookies(u) {
		req.AddCookie(cookie)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// OK
	default:
		return "", fmt.Errorf("failed to get authentication token: %q", resp.Status)
	}

	var tokenResponse = struct {
		Token string
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}
	resp.Body.Close()

	return tokenResponse.Token, nil
}

func buildAuthenticationURL(challengeParams map[string]string, imageName string) (string, error) {
	realm := challengeParams["realm"]
	service := challengeParams["service"]
	scope := challengeParams["scope"]

	realmURL, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	realmURL.RawQuery = fmt.Sprintf("service=%s", service)

	if scope == "" {
		// Default scope for the image we're targeting.
		scope = fmt.Sprintf("repository:%s:pull", imageName)
	}
	realmURL.RawQuery = fmt.Sprintf("%s&scope=%s", realmURL.RawQuery, scope)
	return realmURL.String(), nil
}
