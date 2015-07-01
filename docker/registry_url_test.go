// Copyright 2015 Apcera Inc. All rights reserved.

package docker

import (
	"fmt"
	"testing"
)

type testParseURLValues struct {
	input               string
	expectedError       error
	expectedRegistryURL *DockerRegistryURL
}

type testSchemeHostPortValues struct {
	input  string
	output string
}

type testFullRepoNameValues struct {
	input  string
	output string
}

type testStringValues struct {
	input  *DockerRegistryURL
	output string
}

type testPathValues struct {
	input  *DockerRegistryURL
	output string
}

func TestParseDockerRegistryURL(t *testing.T) {
	testValues := []testParseURLValues{
		// Make sure a full URL still works
		{
			"https://registry-1.docker.io:5000/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"repo",
			nil,
			&DockerRegistryURL{
				ImageName: "repo",
			},
		},
		{
			"namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"repo:tag",
			nil,
			&DockerRegistryURL{
				ImageName: "repo",
				Tag:       "tag",
			},
		},
	}
	for i, val := range testValues {
		result, err := ParseDockerRegistryURL(val.input)
		if err != nil {
			if val.expectedError == nil {
				t.Errorf("Case %d: Unexpected error while parsing struct", i)
			}
			if err.Error() != val.expectedError.Error() {
				t.Errorf("Case %d: Actual error %s does not match expected error %s", i, err, val.expectedError)
			}
			// Error was expected and matched, don't go on to check the result
			// because it is likely to not be relevant.
			continue
		}
		checkURL(t, result, val.expectedRegistryURL)
	}
}

func TestParseFullDockerRegistryURL(t *testing.T) {
	// GOODNESS
	// <scheme>://[user:password@]<host>[:<port>][/<namespace>/<repo>[:<tag>]]
	//
	// BADNESS
	// <scheme>
	// <host>
	// ... And any combination of just scheme or host with others
	// <scheme>://<host>/:<tag>

	testValues := []testParseURLValues{
		{
			"https://registry-1.docker.io",
			nil,
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
			},
		},
		{
			"https://registry-1.docker.io/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				ImageName: "repo",
			},
		},
		{
			"https://registry-1.docker.io/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				ImageName: "repo",
				Tag:       "tag",
			},
		},
		{
			"https://registry-1.docker.io/namespace/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				ImageName: "namespace/repo",
			},
		},
		{
			"https://registry-1.docker.io/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"https://registry-1.docker.io:5000",
			nil,
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
				Port:   "5000",
			},
		},
		{
			"https://registry-1.docker.io:5000/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "repo",
			},
		},
		{
			"https://registry-1.docker.io:5000/namespace/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
		},
		{
			"https://registry-1.docker.io:5000/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		// Test all cases of username:password
		{
			"https://user:password@registry-1.docker.io:5000",
			nil,
			&DockerRegistryURL{
				Scheme:   "https",
				Userinfo: "user:password",
				Host:     "registry-1.docker.io",
				Port:     "5000",
			},
		},
		{
			"https://user:password@registry-1.docker.io:5000/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "user:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "repo",
			},
		},
		{
			"https://user:password@registry-1.docker.io:5000/namespace/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "user:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
		},
		{
			"https://user:password@registry-1.docker.io:5000/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "user:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},

		// Check that trailing slashes parse correctly.
		{
			"https://user:password@registry-1.docker.io:5000/namespace/repo/",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "user:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
		},
		{
			"https",
			fmt.Errorf(`Registry URL must provide a scheme and host: "%s"`, "https"),
			&DockerRegistryURL{},
		},
		{
			"registry-1.docker.io",
			fmt.Errorf(`Registry URL must provide a scheme and host: "%s"`, "registry-1.docker.io"),
			&DockerRegistryURL{},
		},
		{
			"https://registry-1.docker.io/:tag",
			fmt.Errorf(`Path cannot be made up of just a tag: "%s"`, ":tag"),
			&DockerRegistryURL{},
		},
	}

	for i, val := range testValues {
		result, err := ParseFullDockerRegistryURL(val.input)
		if err != nil {
			if val.expectedError == nil {
				t.Errorf("Case %d: Unexpected error while parsing struct", i)
			}
			if err.Error() != val.expectedError.Error() {
				t.Errorf("Case %d: Actual error %s does not match expected error %s", i, err, val.expectedError)
			}
			// Error was expected and matched, don't go on to check the result
			// because it is likely to not be relevant.
			continue
		}
		checkURL(t, result, val.expectedRegistryURL)
	}

}

func TestSchemeHostPort(t *testing.T) {
	testValues := []testSchemeHostPortValues{
		{
			"https://user:password@registry-1.docker.io/namespace/repo:tag",
			"https://user:password@registry-1.docker.io",
		},
		{
			"https://registry-1.docker.io/namespace/repo:tag",
			"https://registry-1.docker.io",
		},
	}

	for _, val := range testValues {
		registryURL, err := ParseFullDockerRegistryURL(val.input)
		if err != nil {
			t.Errorf("Error while parsing input URL: %s", val.input)
		}
		result := registryURL.SchemeHostPort()
		if result != val.output {
			t.Errorf("Result from SchemeHostPort: %s not equal to expected: %s", result, val.output)
		}
	}
}

func TestName(t *testing.T) {
	testValues := []testFullRepoNameValues{
		{
			"https://registry-1.docker.io/namespace/repo:tag",
			"namespace/repo",
		},
		{
			"https://registry-1.docker.io/repo:tag",
			"repo",
		},
	}

	for _, val := range testValues {
		registryURL, err := ParseFullDockerRegistryURL(val.input)
		if err != nil {
			t.Errorf("Error while parsing input URL: %s", val.input)
		}
		if registryURL.ImageName != val.output {
			t.Errorf("Result from SchemeHostPort: %s not equal to expected: %s", registryURL.ImageName, val.output)
		}
	}
}

func TestDockerRegistryURLPath(t *testing.T) {
	testValues := []testPathValues{
		{
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
			},
			"",
		},
		{
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
				Port:   "5000",
			},
			"",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "repo",
			},
			"repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
			"namespace/repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"namespace/repo:tag",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "username:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"namespace/repo:tag",
		},
	}

	for _, val := range testValues {
		result := val.input.Path()
		if result != val.output {
			t.Errorf("Error: expected result %s not equal to actual %s", val.output, result)
		}
	}
}

func TestDockerRegistryURLString(t *testing.T) {
	testValues := []testStringValues{
		{
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
			},
			"https://registry-1.docker.io",
		},
		{
			&DockerRegistryURL{
				Scheme: "https",
				Host:   "registry-1.docker.io",
				Port:   "5000",
			},
			"https://registry-1.docker.io:5000",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "repo",
			},
			"https://registry-1.docker.io:5000/repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
			"https://registry-1.docker.io:5000/namespace/repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"https://registry-1.docker.io:5000/namespace/repo:tag",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  "username:password",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"https://username:password@registry-1.docker.io:5000/namespace/repo:tag",
		},
	}

	for _, val := range testValues {
		result := val.input.String()
		if result != val.output {
			t.Errorf("Error: expected result %s not equal to actual %s", val.output, result)
		}
	}
}

func TestClearUserCredentials(t *testing.T) {
	registryURL := DockerRegistryURL{
		Userinfo: "user:password",
	}

	registryURL.ClearUserCredentials()

	if registryURL.Userinfo != "" {
		t.Errorf("ClearUserCredentials did not clear Userinfo field.")
	}
}

// HELPERS

func checkURL(t *testing.T, actualURL, expectedURL *DockerRegistryURL) {
	if actualURL.Scheme != expectedURL.Scheme {
		t.Fatalf("actualURL.Scheme %s does not match assertion: %s", actualURL.Scheme, expectedURL.Scheme)
	}
	if actualURL.Userinfo != expectedURL.Userinfo {
		t.Fatalf("actualURL.Userinfo %s does not match assertion: %s", actualURL.Userinfo, expectedURL.Userinfo)
	}
	if actualURL.Host != expectedURL.Host {
		t.Fatalf("actualURL.Host %s does not match assertion: %s", actualURL.Host, expectedURL.Host)
	}
	if actualURL.Port != expectedURL.Port {
		t.Fatalf("actualURL.Port %s does not match assertion: %s", actualURL.Port, expectedURL.Port)
	}
	if actualURL.ImageName != expectedURL.ImageName {
		t.Fatalf("actualURL.ImageName %s does not match assertion: %s", actualURL.ImageName, expectedURL.ImageName)
	}
	if actualURL.Tag != expectedURL.Tag {
		t.Fatalf("actualURL.Tag %s does not match assertion: %s", actualURL.Tag, expectedURL.Tag)
	}
}
