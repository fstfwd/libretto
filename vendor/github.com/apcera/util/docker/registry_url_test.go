// Copyright 2015-2016 Apcera Inc. All rights reserved.

package docker

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"testing"
)

func TestParseDockerRegistryURL(t *testing.T) {
	testValues := []struct {
		input               string
		expectedError       error
		expectedRegistryURL *DockerRegistryURL
	}{
		{
			"https://user:test%2fpass@registry-1.docker.io:5000/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
				Userinfo:  url.UserPassword("user", "test/pass"),
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
		{
			"registry-1.docker.io:5000/namespace/repo:tag",
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
			"http://registry-1.docker.io:5000/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "http",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"http://registry-1.docker.io/namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "http",
				Host:      "registry-1.docker.io",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				ImageName: "repo",
			},
		},
		{
			"namespace/repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
		},
		{
			"docker-registry.apcera.net/alex/ubuntu-img:15.10",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "docker-registry.apcera.net",
				ImageName: "alex/ubuntu-img",
				Tag:       "15.10",
			},
		},
		{
			"repo:tag",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				ImageName: "repo",
				Tag:       "tag",
			},
		},
		{
			"httpd",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				ImageName: "httpd",
			},
		},
		{
			"some/weird/:image",
			fmt.Errorf(`Image name must not have a trailing "/": weird/`),
			&DockerRegistryURL{},
		},
	}

	for i, val := range testValues {
		result, err := ParseDockerRegistryURL(val.input)
		if err != nil && val.expectedError != nil && err.Error() == val.expectedError.Error() {
			continue
		} else if err != nil && val.expectedError != nil && err.Error() != val.expectedError.Error() {
			t.Errorf("Case %d: Actual error %q does not match expected error: %q", i, err, val.expectedError)
			continue
		} else if err != nil && val.expectedError == nil {
			t.Errorf("Case %d: Unexpected error while parsing struct: %s", i, err)
			continue
		} else if err == nil && val.expectedError != nil {
			t.Errorf("Expected an error but didn't get one: %s", val.expectedError)
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

	testValues := []struct {
		input               string
		expectedError       error
		expectedRegistryURL *DockerRegistryURL
	}{
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
			"quay.io/namespace/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "quay.io",
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
				Userinfo: url.UserPassword("user", "password"),
				Host:     "registry-1.docker.io",
				Port:     "5000",
			},
		},
		{
			"https://user:password@registry-1.docker.io:5000/repo",
			nil,
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  url.UserPassword("user", "password"),
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
				Userinfo:  url.UserPassword("user", "password"),
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
				Userinfo:  url.UserPassword("user", "password"),
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
				Userinfo:  url.UserPassword("user", "password"),
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
		{
			"http://127.0.0.1:49375/some/weird/:image",
			fmt.Errorf(`Image name must not have a trailing "/": some/weird/`),
			&DockerRegistryURL{},
		},
	}

	for i, val := range testValues {
		result, err := ParseFullDockerRegistryURL(val.input)
		if err != nil && val.expectedError != nil && err.Error() == val.expectedError.Error() {
			continue
		} else if err != nil && val.expectedError != nil && err.Error() != val.expectedError.Error() {
			t.Errorf("Case %d: Actual error %s does not match expected error %s", i, err, val.expectedError)
			// Error was expected and matched, don't go on to check the result
			// because it is likely to not be relevant.
			continue
		} else if err != nil && val.expectedError == nil {
			t.Errorf("Case %d: Unexpected error while parsing struct: %s", i, err)
			continue
		} else if err == nil && val.expectedError != nil {
			t.Errorf("Expected an error but didn't get one: %s", val.expectedError)
			continue
		}
		checkURL(t, result, val.expectedRegistryURL)
	}
}

func TestBaseURL(t *testing.T) {
	testValues := []struct {
		input  string
		output string
	}{
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
		result := registryURL.BaseURL()
		if result != val.output {
			t.Errorf("Result from BaseURL: %s not equal to expected: %s", result, val.output)
		}
	}
}

func TestBaseURLNoCredentials(t *testing.T) {
	testValues := []struct {
		input  string
		output string
	}{
		{
			"https://user:password@registry-1.docker.io/namespace/repo:tag",
			"https://registry-1.docker.io",
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
		result := registryURL.BaseURLNoCredentials()
		if result != val.output {
			t.Errorf("Result from BaseURLNoCredentials: %s not equal to expected: %s", result, val.output)
		}
	}
}

func TestName(t *testing.T) {
	testValues := []struct {
		input  string
		output string
	}{
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
			t.Errorf("Result from ImageName: %s not equal to expected: %s", registryURL.ImageName, val.output)
		}
	}
}

func TestDockerRegistryURLPath(t *testing.T) {
	testValues := []struct {
		input  *DockerRegistryURL
		output string
	}{
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
				Userinfo:  url.UserPassword("user", "password"),
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
	testValues := []struct {
		input  *DockerRegistryURL
		output string
	}{
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
				Userinfo:  url.UserPassword("user", "password"),
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"https://user:password@registry-1.docker.io:5000/namespace/repo:tag",
		},
	}

	for _, val := range testValues {
		result := val.input.String()
		if result != val.output {
			t.Errorf("Error: expected result %s not equal to actual %s", val.output, result)
		}
	}
}

func TestDockerRegistryURLStringNoCredentials(t *testing.T) {
	testValues := []struct {
		input  *DockerRegistryURL
		output string
	}{
		{
			&DockerRegistryURL{
				Scheme:   "https",
				Userinfo: url.UserPassword("user", "password"),
				Host:     "registry-1.docker.io",
			},
			"https://registry-1.docker.io",
		},
		{
			&DockerRegistryURL{
				Scheme:   "https",
				Userinfo: url.UserPassword("user", "password"),
				Host:     "registry-1.docker.io",
				Port:     "5000",
			},
			"https://registry-1.docker.io:5000",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  url.UserPassword("user", "password"),
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "repo",
			},
			"https://registry-1.docker.io:5000/repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  url.UserPassword("user", "password"),
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
			},
			"https://registry-1.docker.io:5000/namespace/repo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Userinfo:  url.UserPassword("user", "password"),
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"https://registry-1.docker.io:5000/namespace/repo:tag",
		},
	}

	for _, val := range testValues {
		result := val.input.StringNoCredentials()
		if result != val.output {
			t.Errorf("Error: expected result %s not equal to actual %s", val.output, result)
		}
	}
}

func TestClearUserCredentials(t *testing.T) {
	registryURL := DockerRegistryURL{
		Userinfo: url.UserPassword("user", "password"),
	}

	registryURL.ClearUserCredentials()

	if registryURL.Userinfo != nil {
		t.Errorf("ClearUserCredentials did not clear Userinfo field.")
	}
}

// TestMarshalJSON_Userinfo marhals and unmarshals the JSON to ensure that userinfo is
// handled properly.
func TestMarshalJSON_Userinfo(t *testing.T) {
	testValues := []struct {
		input *DockerRegistryURL
		msg   string
	}{
		{
			nil,
			"nil",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
			},
			"Basic info without userinfo",
		},
		{
			&DockerRegistryURL{
				Scheme:    "https",
				Host:      "registry-1.docker.io",
				Port:      "5000",
				ImageName: "namespace/repo",
				Tag:       "tag",
				Userinfo:  url.UserPassword("user", "pass"),
			},
			"Basic info simple userinfo",
		},
		{
			&DockerRegistryURL{
				Scheme:   "https",
				Host:     "registry-1.docker.io",
				Userinfo: url.UserPassword("user", "pass/word"),
			},
			"Basic info special chars in userinfo",
		},
		{
			&DockerRegistryURL{
				Scheme:   "https",
				Host:     "registry-1.docker.io",
				Userinfo: url.User("user"),
			},
			"Basic info with just user",
		},
	}

	for _, val := range testValues {
		jsonS, err := json.Marshal(val.input)
		if err != nil {
			t.Fatalf("Unexpected error while marshalling data: %v Data: %v", err, val.input)
		}
		var newVal *DockerRegistryURL = nil
		err = json.Unmarshal(jsonS, &newVal)
		if err != nil {
			t.Fatalf("Unexpected error while unmarshalling data: %v Data: %v", err, string(jsonS))
		}

		if !reflect.DeepEqual(val.input, newVal) {
			t.Errorf("Expected %+v but got %+v instead.", val.input, newVal)
		}
	}
}

// HELPERS

func checkURL(t *testing.T, actualURL, expectedURL *DockerRegistryURL) {
	if actualURL.Scheme != expectedURL.Scheme {
		t.Errorf("actualURL.Scheme %q does not match assertion: %q", actualURL.Scheme, expectedURL.Scheme)
	}
	if actualURL.Userinfo != nil && expectedURL.Userinfo == nil ||
		actualURL.Userinfo == nil && expectedURL.Userinfo != nil ||
		(actualURL.Userinfo != nil && expectedURL.Userinfo != nil &&
			actualURL.Userinfo.String() != expectedURL.Userinfo.String()) {
		t.Errorf("actualURL.Userinfo %q does not match assertion: %q", actualURL.Userinfo, expectedURL.Userinfo)
	}
	if actualURL.Host != expectedURL.Host {
		t.Errorf("actualURL.Host %q does not match assertion: %q", actualURL.Host, expectedURL.Host)
	}
	if actualURL.Port != expectedURL.Port {
		t.Errorf("actualURL.Port %q does not match assertion: %q", actualURL.Port, expectedURL.Port)
	}
	if actualURL.ImageName != expectedURL.ImageName {
		t.Errorf("actualURL.ImageName %q does not match assertion: %q", actualURL.ImageName, expectedURL.ImageName)
	}
	if actualURL.Tag != expectedURL.Tag {
		t.Errorf("actualURL.Tag %q does not match assertion: %q", actualURL.Tag, expectedURL.Tag)
	}
}
