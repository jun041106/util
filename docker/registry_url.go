// Copyright 2015 Apcera Inc. All rights reserved.

package docker

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// DockerRegistryURL represents all components of a Docker V1 Registry URL. See
// this link for more information about the DockerHub and V1 registry API:
//
// https://docs.docker.com/reference/api/docker-io_api/
//
// Mandatory fields for a valid RegistryURL:
// Scheme, Host
//
// Possible formats:
//
// <scheme>://[user:password@]<host>[:<port>][/<namespace>/<repo>[:<tag>]]
type DockerRegistryURL struct {
	// Scheme can be http or https.
	Scheme string `json:",omitempty"`

	// Userinfo holds basic auth credentials.
	Userinfo string `json:",omitempty"`

	// Host is a Fully Qualified Domain Name.
	Host string `json:",omitempty"`

	// Port is optional, and may not be present.
	Port string `json:",omitempty"`

	// ImageName is an image repository. The ImageName can be just the Repo name,
	// or can also have arbitrary nesting of namespaces (e.g. namespace1/namespace2/repo).
	// This field is optional when specifying Docker registry source whitelists.
	ImageName string `json:",omitempty"`

	// Tag specifies a desired version of the docker image. For instance, to
	// specify ubuntu 14.04, the tag is 14.04.
	Tag string `json:",omitempty"`
}

// ParseDockerRegistryURL parses a Docker Registry URL. It does not need to be
// a full URL. If the url starts with either http or https, the input path will
// be passed to ParseFullDockerRegistryURL. If not, it will try to parse the
// URL as if it is a (namespace/)*repo(:tag)?
func ParseDockerRegistryURL(s string) (*DockerRegistryURL, error) {
	registryURL, err := ParseFullDockerRegistryURL(s)
	if err == nil {
		return registryURL, nil
	}
	// String didn't parse but was supposed to be a full registry URL.
	if strings.HasPrefix(s, "http") || strings.HasPrefix(s, "https") {
		return nil, fmt.Errorf("Invalid full Docker registry URL: %s", err)
	}

	registryURL = &DockerRegistryURL{}
	if err = registryURL.parsePath(s); err != nil {
		return nil, err
	}
	return registryURL, nil
}

// ParseFullDockerRegistryURL validates an input string URL to make sure
// that it conforms to the Docker V1 registry URL schema.
func ParseFullDockerRegistryURL(s string) (*DockerRegistryURL, error) {
	registryURL := &DockerRegistryURL{}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" || u.Host == "" {
		return registryURL, fmt.Errorf("Registry URL must provide a scheme and host: %q", s)
	}

	registryURL.Scheme = u.Scheme

	if u.User != nil {
		registryURL.Userinfo = u.User.String()
	}

	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	registryURL.Host = host
	registryURL.Port = port

	// Parse everything after <scheme>://<host>:<port>
	err = registryURL.parsePath(u.Path)
	if err != nil {
		return nil, err
	}
	return registryURL, nil
}

// splitHostPort wraps net.SplitHostPort, and can be called on a host
// whether or not it contains a port segment.
func splitHostPort(hostport string) (string, string, error) {
	if strings.Contains(hostport, ":") {
		return net.SplitHostPort(hostport)
	} else {
		return hostport, "", nil
	}
}

// parsePath parses the Registry URL path (after <scheme>:://<host>:<port>)
// into namespace, repo, and tag. All of these are optional parts of the
// path.
func (url *DockerRegistryURL) parsePath(s string) error {
	s, err := cleanPath(s)
	if err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	imageName, tag, err := parseTag(s)
	if err != nil {
		return err
	}
	url.Tag = tag
	url.ImageName = imageName

	return nil
}

// cleanPath removes leading and trailing forward slashes
// and makes sure that the path does not only contain a tag.
func cleanPath(s string) (string, error) {
	s = strings.Trim(s, "/")
	if strings.HasPrefix(s, ":") {
		return "", fmt.Errorf("Path cannot be made up of just a tag: %q", s)
	}
	return s, nil
}

// parseTag splits the Repository tag from the prefix.
func parseTag(s string) (prefix, tag string, err error) {
	splitString := strings.Split(s, ":")
	if len(splitString) == 1 {
		prefix = splitString[0]
	} else if len(splitString) == 2 {
		prefix, tag = splitString[0], splitString[1]
	} else {
		// Unlikely edge case but it doesn't hurt to test for it.
		return "", "", fmt.Errorf("Path should not contain more than one colon: %q", s)
	}
	return prefix, tag, nil
}

// SchemeHostPort (for lack of a better name)returns a string of everything
// before the path in the DockerRegistryURL. The following format applies:
// <scheme>://[username:password@]<FQDN>[:port]
func (u *DockerRegistryURL) SchemeHostPort() string {
	if u.Scheme == "" || u.Host == "" {
		return ""
	}
	var result string
	if u.Userinfo != "" {
		result = fmt.Sprintf("%s://%s@%s", u.Scheme, u.Userinfo, u.Host)
	} else {
		result = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
	if u.Port != "" {
		result = fmt.Sprintf("%s:%s", result, u.Port)
	}
	return result
}

// Path returns the string path segment of a DockerRegistryURL. The full format
// of a path is [namespace/]repo[:tag].
func (u *DockerRegistryURL) Path() string {
	if u.Tag == "" {
		return u.ImageName
	} else {
		return fmt.Sprintf("%s:%s", u.ImageName, u.Tag)
	}
}

// String returns the full string version of a DockerRegistryURL
func (u *DockerRegistryURL) String() string {
	schemeHostPort := u.SchemeHostPort()
	if schemeHostPort == "" {
		return ""
	}

	s := u.Path()
	if s == "" {
		return schemeHostPort
	} else {
		return fmt.Sprintf("%s/%s", schemeHostPort, s)
	}
}

// ClearUserCredentials will remove any Userinfo from a provided DockerRegistryURL object.
func (u *DockerRegistryURL) ClearUserCredentials() {
	u.Userinfo = ""
}
