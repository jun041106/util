// Copyright 2015 Apcera Inc. All rights reserved.

package v2

import (
	"fmt"
	"net/http"
	"testing"

	// TODO: godep?
	"github.com/docker/distribution/registry/client/auth"
)

func TestDocker_BuildAuthenticationURL(t *testing.T) {
	expectedRealm := "https://auth.docker.io/token"
	expectedService := "registry.docker.io"

	imageName := "library/nats"
	expectedScope := "repository:" + imageName

	challenge := fmt.Sprintf("Bearer realm=%q,service=%q,scope=%q",
		expectedRealm,
		expectedService,
		expectedScope)

	resp := &http.Response{
		Header:     http.Header{},
		StatusCode: http.StatusUnauthorized,
	}
	resp.Header.Set("WWW-Authenticate", challenge)

	challenges := auth.ResponseChallenges(resp)
	if len(challenges) != 1 {
		t.Fatalf("Got %d challenges; expected 1", len(challenges))
	}

	expectedAuthURL := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/nats"

	authURL, err := buildAuthenticationURL(challenges[0].Parameters, imageName)
	if err != nil {
		t.Fatalf("Failed to build auth URL: %s", err)
	}

	if authURL != expectedAuthURL {
		t.Fatalf("Expected %q, got %q", expectedAuthURL, authURL)
	}
}

// TODO: add more tests
