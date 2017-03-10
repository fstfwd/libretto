// Copyright 2015-2016 Apcera Inc. All rights reserved.

package docker

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path"
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

	// Userinfo holds basic auth credentials. If you want to use a preformed
	// token for authentication, it should be decoded from base64 and parsed
	// into the url.Userinfo type.
	Userinfo *url.Userinfo `json:",omitempty"`

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
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return nil, fmt.Errorf("Invalid full Docker registry URL: %s", err)
	}

	// At this point we have no scheme, possibly a host, possibly a port,
	// possibly a repo, possibly an image name, and possibly a tag. We want to
	// expand the Docker registry URL here, using HTTPS as a default scheme.
	registryURL = &DockerRegistryURL{
		Scheme: "https",
	}

	registryURLParts := strings.Split(s, "/")
	if len(registryURLParts) == 3 {
		// host[:port]/[repo/]image/[:tag]
		host, port, err := splitHostPort(registryURLParts[0])
		if err != nil {
			return nil, err
		}
		registryURL.Host = host
		registryURL.Port = port
		s = path.Join(registryURLParts[1], registryURLParts[2])
	}

	if err = registryURL.parsePath(s); err != nil {
		return nil, err
	}
	return registryURL, nil
}

// ParseFullDockerRegistryURL validates an input string URL to make sure
// that it conforms to the Docker V1 registry URL schema.
func ParseFullDockerRegistryURL(s string) (*DockerRegistryURL, error) {
	s = handleSchemelessQuayRegistries(s)

	registryURL := &DockerRegistryURL{
		Scheme: "https",
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" || u.Host == "" {
		return registryURL, fmt.Errorf("Registry URL must provide a scheme and host: %q", s)
	}

	registryURL.Scheme = u.Scheme
	registryURL.Userinfo = u.User

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

// handleSchemelessQuayRegistries handles scheme-less Quay URLs provided as Quay
// lists on their repository pages. No-op for non-Quay registries, or for Quay
// URLs provdied with a scheme.
func handleSchemelessQuayRegistries(s string) string {
	if !strings.HasPrefix(s, "quay.io") {
		return s
	}

	return "https://" + s
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

// parsePath parses the Registry URL path (after <scheme>://<host>:<port>)
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

	if strings.HasSuffix(prefix, "/") {
		return "", "", fmt.Errorf(`Image name must not have a trailing "/": %s`, prefix)
	}

	return prefix, tag, nil
}

// baseURL is wrapped by BaseURL and BaseURLNoCredentials.
// The boolean flag indicates whether credentials are included in the return string.
func (u *DockerRegistryURL) baseURL(includeCredentials bool) string {
	if u.Scheme == "" || u.Host == "" {
		return ""
	}

	var result string
	if u.Userinfo != nil && includeCredentials {
		result = fmt.Sprintf("%s://%s@%s", u.Scheme, u.Userinfo, u.Host)
	} else {
		result = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	if u.Port != "" {
		result = fmt.Sprintf("%s:%s", result, u.Port)
	}
	return result

}

// HostPort returns the HostPort of the registry URL. If there is no port, then
// it returns just the host.
func (u *DockerRegistryURL) HostPort() string {
	if u.Port == "" {
		return u.Host
	}
	return net.JoinHostPort(u.Host, u.Port)

}

// AddLibraryNamespace explicitly appends the `library` namespace used for
// official images.
func (u *DockerRegistryURL) AddLibraryNamespace() {
	// If the image name does not contain '/', it is intended to be an official
	// image; so we add the `library` namespace to the URL.
	if !strings.Contains(u.ImageName, "/") {
		u.ImageName = fmt.Sprintf("library/%s", u.ImageName)
	}
}

// BaseURL returns a string of the format: <scheme>://(<creds>@)?<host>(:<port>)?
func (u *DockerRegistryURL) BaseURL() string {
	return u.baseURL(true)
}

// BaseURLNoCredentials returns a string of the format: <scheme://host(:port)?
func (u *DockerRegistryURL) BaseURLNoCredentials() string {
	return u.baseURL(false)
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

// baseString is wrapped by String and StringNoCredentials.
// It exposes a flag to indicate whether to include credentials in the BaseURL.
func (u *DockerRegistryURL) baseString(includeCredentials bool) string {
	var base string
	if includeCredentials {
		base = u.BaseURL()
	} else {
		base = u.BaseURLNoCredentials()
	}

	s := u.Path()
	if s == "" {
		return base
	} else {
		return fmt.Sprintf("%s/%s", base, s)
	}
}

// String returns the full form of a registryURL.
func (u *DockerRegistryURL) String() string {
	return u.baseString(true)
}

// StringNoCredentials returns the full form of a registryURL without credentials.
func (u *DockerRegistryURL) StringNoCredentials() string {
	return u.baseString(false)
}

// ClearUserCredentials will remove any Userinfo from a provided DockerRegistryURL object.
func (u *DockerRegistryURL) ClearUserCredentials() {
	u.Userinfo = nil
}

// MarshalJSON implements marshalling which will maintain the url.Userinfo
// details which are normally lost due to lack of exported fields.
func (u *DockerRegistryURL) MarshalJSON() ([]byte, error) {
	// Using the alias type allows it to have all the same fields as the
	// DockerRegistryURL but avoids a loop when marshalling since it will not
	// have the same MarshalJSON method.
	type Alias DockerRegistryURL
	ui := ""
	if u != nil && u.Userinfo != nil {
		ui = u.Userinfo.String()
	}
	return json.Marshal(&struct {
		Userinfo string `json:",omitempty"`
		*Alias
	}{
		Userinfo: ui,
		Alias:    (*Alias)(u),
	})
}

// UnmarshalJSON implements unmarshalling which will maintain the url.Userinfo
// details which are normally lost due to lack of exported fields.
func (u *DockerRegistryURL) UnmarshalJSON(data []byte) error {
	// Using the alias type allows it to have all the same fields as the
	// DockerRegistryURL but avoids a loop when marshalling since it will not
	// have the same UnmarshalJSON method.
	type Alias DockerRegistryURL
	aux := &struct {
		Userinfo string `json:",omitempty"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Userinfo == "" {
		return nil
	}
	ui, err := url.Parse("http://" + aux.Userinfo + "@foo.com")
	if err != nil {
		return fmt.Errorf("error parsing userinfo: %s", err)
	}
	u.Userinfo = ui.User
	return nil
}
