// Copyright 2015 Apcera Inc. All rights reserved.

package v2

import (
	"fmt"
	"testing"

	v2mock "github.com/apcera/util/dockertest/v2"
)

func init() {
	v2Registry := v2mock.RunMockRegistry()
	DockerHubRegistryURL = v2Registry.URL
}

func TestFetchImage_NoAuth(t *testing.T) {
	imageName := "library/nats:latest"
	unparsedDockerURL := fmt.Sprintf("%s/%s", DockerHubRegistryURL, imageName)

	// Disable authentication here.
	v2mock.SetSkipAuth(true)
	defer v2mock.SetSkipAuth(false)

	dockerClient, err := NewDockerClientFromRawURL(unparsedDockerURL)
	if err != nil {
		t.Fatalf("Failed to create Docker client for %q: %s", unparsedDockerURL, err)
	}

	layers, err := dockerClient.FetchImage()
	if err != nil {
		t.Fatalf("Failed to fetch layers of image %q: %s", unparsedDockerURL, err)
	}

	if len(layers) != 6 {
		t.Fatalf("Expected 6 layers, got %d", len(layers))
	}
}

// TODO: add more tests
