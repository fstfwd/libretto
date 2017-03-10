// Copyright 2014-2015 Apcera Inc. All rights reserved.

// v1 is a Docker v1 Registry API client implementation. The v1 API has
// been deprecated by the public Docker Hub as of December 7th, 2015.
//
// See: https://docs.docker.com/v1.6/reference/api/registry_api/
package v1

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

var (
	// DockerHubRegistryURL points to the official Docker registry.
	DockerHubRegistryURL = "https://index.docker.io"
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

// GetImage fetches Docker repository information from the specified Docker
// registry. If the registry is an empty string it defaults to the DockerHub.
// The integer return value is the status code of the HTTP response.
func GetImage(name, registryURL string) (*Image, int, error) {
	if name == "" {
		return nil, -1, errors.New("image name is empty")
	}

	var ru *url.URL
	var err error
	if len(registryURL) != 0 {
		ru, err = url.Parse(registryURL)
	} else {
		ru, err = url.Parse(DockerHubRegistryURL)
		registryURL = DockerHubRegistryURL
	}
	if err != nil {
		return nil, -1, err
	}

	// In order to get layers from Docker CDN we need to hit 'images' endpoint
	// and request the token. Client should also accept and store cookies, as
	// they are needed to fetch the layer data later.
	imagesURL := fmt.Sprintf("%s/v1/repositories/%s/images", registryURL, name)

	req, err := http.NewRequest("GET", imagesURL, nil)
	if err != nil {
		return nil, -1, err
	}

	// FIXME: Send 'Connection: close' header on HTTP requests
	// as reusing the connection is still prone to EOF/Connection reset errors
	// in certain network environments...
	// See: ENGT-9670
	// https://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors
	req.Close = true

	req.Header.Set("X-Docker-Token", "true")

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
	client.Jar, err = cookiejar.New(nil) // Docker repo API sets and uses cookies for CDN.
	if err != nil {
		return nil, -1, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, -1, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		// Fall through.
	case http.StatusNotFound:
		return nil, res.StatusCode, fmt.Errorf("image %q not found", name)
	default:
		return nil, res.StatusCode, fmt.Errorf("HTTP %d ", res.StatusCode)
	}

	token := res.Header.Get("X-Docker-Token")
	endpoints := strings.Split(res.Header.Get("X-Docker-Endpoints"), ",")

	if len(endpoints) == 0 {
		return nil, res.StatusCode, errors.New("Docker index response didn't contain any endpoints")
	}
	for i := range endpoints {
		endpoints[i] = strings.Trim(endpoints[i], " ")
	}

	img := &Image{
		Name:      name,
		client:    client,
		endpoints: endpoints,
		token:     token,
		scheme:    ru.Scheme,
	}

	img.tags, err = img.fetchTags()
	if err != nil {
		return nil, res.StatusCode, err
	}

	return img, res.StatusCode, nil
}

// Tags returns a list of tags available for image
func (i *Image) Tags() []string {
	result := make([]string, 0)

	for tag, _ := range i.tags {
		result = append(result, tag)
	}

	return result
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

// LayerURLs returns several URLs for a specific layer.
func (i *Image) LayerURLs(id string) []string {
	var urls []string
	for _, ep := range i.endpoints {
		urls = append(urls, fmt.Sprintf("%s://%s/v1/images/%s/layer", i.scheme, ep, id))
	}
	return urls
}

// AuthorizationHeader exposes the authorization header created for the image
// for external layer downloads.
func (i *Image) AuthorizationHeader() string {
	if i.token == "" {
		return ""
	}
	return fmt.Sprintf("Token %s", i.token)
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
		defer res.Body.Close()
		type errorMsg struct {
			Error string `json:"error"`
		}

		var errMsg errorMsg
		if err := json.NewDecoder(res.Body).Decode(&errMsg); err == nil {
			return nil, fmt.Errorf("%s: HTTP %d - %s", u, res.StatusCode, errMsg.Error)
		}

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

// Cookie returns the string representation of the first
// cookie stored in stored client's cookie jar.
func (i *Image) Cookie(u string) (string, error) {
	if i.client.Jar == nil {
		return "", nil
	}

	baseURL, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("Invalid URL: %s", err)
	}

	cookies := i.client.Jar.Cookies(baseURL)
	if len(cookies) == 0 {
		return "", nil
	}

	return cookies[0].String(), nil
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
