// Copyright 2013-2014 Apcera Inc. All rights reserved.

// Package restclient wraps a REST-ful web service to expose objects from the
// service in Go programs. Construct a client using
// restclient.New("http://service.com/api/endpoint"). Use the client's HTTP-verb
// methods to receive the result of REST operations in a Go type. For example,
// to get a collection of Items, invoke client.Get("items", m) where m is of
// type []Item.
//
// The package also exposes lower level interfaces to receive the raw
// http.Response from the client and to construct requests to a client's service
// that may be sent later, or by an alternate client or transport.
package restclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Method wraps HTTP verbs for stronger typing.
type Method string

// HTTP methods for REST
const (
	GET    = Method("GET")
	POST   = Method("POST")
	PUT    = Method("PUT")
	DELETE = Method("DELETE")
)

const (
	vndMediaTypePrefix  = "application/vnd"
	mediaTypeJSONSuffix = "+json"
)

// Client represents a client bound to a given REST base URL.
type Client struct {
	// Driver is the *http.Client that performs requests.
	Driver *http.Client
	// base is the URL under which all REST-ful resources are available.
	base *url.URL
	// Headers represents common headers that are added to each request.
	Headers http.Header
	// KeepAlives enabled
	KeepAlives bool
}

// New returns a *Client with the specified base URL endpoint, expected to
// include the port string and any path, if required. Returns an error if
// baseurl cannot be parsed as an absolute URL.
func New(baseurl string) (*Client, error) {
	base, err := url.ParseRequestURI(baseurl)
	if err != nil {
		return nil, err
	} else if !base.IsAbs() || base.Host == "" {
		return nil, fmt.Errorf("URL is not absolute: %s", baseurl)
	}

	// create the client
	client := &Client{
		Driver:     &http.Client{}, // Don't use default client; shares by reference
		Headers:    http.Header(make(map[string][]string)),
		base:       base,
		KeepAlives: true,
	}

	return client, nil
}

// NewWithoutKeepAlives returns a new client with keepalives disabled.
func NewDisableKeepAlives(baseurl string) (*Client, error) {
	base, err := url.ParseRequestURI(baseurl)
	if err != nil {
		return nil, err
	} else if !base.IsAbs() || base.Host == "" {
		return nil, fmt.Errorf("URL is not absolute: %s", baseurl)
	}

	transport := http.DefaultTransport.(*http.Transport)
	transport.DisableKeepAlives = true

	// create the client
	client := &Client{
		Driver: &http.Client{
			Transport: transport,
		}, // Don't use default client; shares by reference
		Headers:    http.Header(make(map[string][]string)),
		base:       base,
		KeepAlives: false,
	}

	return client, nil
}

// BaseURL returns a *url.URL to a copy of Client's base so the caller may
// modify it.
func (c *Client) BaseURL() *url.URL {
	return c.base.ResolveReference(&url.URL{})
}

// Set the access Token
func (c *Client) SetAccessToken(token string) {
	c.Headers.Set(http.CanonicalHeaderKey("Authorization"), "Bearer "+token)
}

// SetTimeout sets the timeout of a client to the given duration.
func (c *Client) SetTimeout(duration time.Duration) {
	c.Driver.Timeout = duration
}

// Get issues a GET request to the specified endpoint and parses the response
// into resp. It will return an error if it failed to send the request, a
// *RestError if the response wasn't a 2xx status code, or an error from package
// json's Decode.
func (c *Client) Get(endpoint string, resp interface{}) error {
	return c.Result(c.NewJsonRequest(GET, endpoint, nil), resp)
}

// Post issues a POST request to the specified endpoint with the req payload
// marshaled to JSON and parses the response into resp. It will return an error
// if it failed to send the request, a *RestError if the response wasn't a 2xx
// status code, or an error from package json's Decode.
func (c *Client) Post(endpoint string, req interface{}, resp interface{}) error {
	return c.Result(c.NewJsonRequest(POST, endpoint, req), resp)
}

// Put issues a PUT request to the specified endpoint with the req payload
// marshaled to JSON and parses the response into resp. It will return an error
// if it failed to send the request, a *RestError if the response wasn't a 2xx
// status code, or an error from package json's Decode.
func (c *Client) Put(endpoint string, req interface{}, resp interface{}) error {
	return c.Result(c.NewJsonRequest(PUT, endpoint, req), resp)
}

// Delete issues a DELETE request to the specified endpoint and parses the
// response in to resp. It will return an error if it failed to send the request, a
// *RestError if the response wasn't a 2xx status code, or an error from package
// json's Decode.
func (c *Client) Delete(endpoint string, resp interface{}) error {
	return c.Result(c.NewJsonRequest(DELETE, endpoint, nil), resp)
}

// Result performs the request described by req and unmarshals a successful
// HTTP response into resp. If resp is nil, the response is discarded.
func (c *Client) Result(req *Request, resp interface{}) error {
	result, err := c.Do(req)
	if err != nil {
		return err
	}
	return unmarshal(result, resp)
}

// Do performs the HTTP request described by req and returns the *http.Response.
// Also returns a non-nil *RestError if an error occurs or the response is not
// in the 2xx family.
func (c *Client) Do(req *Request) (*http.Response, error) {
	hreq, err := req.HTTPRequest()
	if err != nil {
		return nil, &RestError{Req: hreq, err: fmt.Errorf("error preparing request: %s", err)}
	}

	if !c.KeepAlives {
		hreq.Close = true
	}

	// Internally, this uses c.Driver's CheckRedirect policy.
	resp, err := c.Driver.Do(hreq)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Timeout() {
				return nil, &RestError{Req: hreq, err: fmt.Errorf("timed out making request")}
			}
		}
		return resp, &RestError{Req: hreq, Resp: resp, err: fmt.Errorf("error sending request: %s", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, &RestError{Req: hreq, Resp: resp, err: fmt.Errorf("error in response: %s", resp.Status)}
	}
	return resp, nil
}

// NewRequest generates a new Request object that will send bytes read from body
// to the endpoint.
func (c *Client) NewRequest(method Method, endpoint string, ctype string, body io.Reader) (req *Request) {
	req = c.newRequest(method, endpoint)
	if body == nil {
		return
	}

	req.prepare = func(hr *http.Request) error {
		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = ioutil.NopCloser(body)
		}
		hr.Body = rc
		hr.Header.Set("Content-Type", ctype)
		return nil
	}
	return
}

// NewJsonRequest generates a new Request object and JSON encodes the provided
// obj. The JSON object will be set as the body and included in the request.
func (c *Client) NewJsonRequest(method Method, endpoint string, obj interface{}) (req *Request) {
	req = c.newRequest(method, endpoint)
	if obj == nil {
		return
	}

	req.prepare = func(httpReq *http.Request) error {
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		if err := encoder.Encode(obj); err != nil {
			return err
		}

		// set to the request
		httpReq.Body = ioutil.NopCloser(&buffer)
		httpReq.ContentLength = int64(buffer.Len())
		httpReq.Header.Set("Content-Type", "application/json")
		return nil
	}

	return req
}

// NewFormRequest generates a new Request object with a form encoded body based
// on the params map.
func (c *Client) NewFormRequest(method Method, endpoint string, params map[string]string) *Request {
	req := c.newRequest(method, endpoint)

	// set how to generate the body
	req.prepare = func(httpReq *http.Request) error {
		form := url.Values{}
		for k, v := range params {
			form.Set(k, v)
		}
		encoded := form.Encode()

		// set to the request
		httpReq.Body = ioutil.NopCloser(bytes.NewReader([]byte(encoded)))
		httpReq.ContentLength = int64(len(encoded))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return nil
	}

	return req
}

// newRequest returns a *Request ready to be used by one of Client's exported
// methods like NewFormRequest.
func (c *Client) newRequest(method Method, endpoint string) *Request {
	req := &Request{
		Method:  method,
		URL:     resourceURL(c.BaseURL(), endpoint),
		Headers: http.Header(make(map[string][]string)),
	}

	// Copy over the headers. Don't set them directly to ensure changing
	// them on the request doesn't change them on the client.
	for k, vv := range c.Headers {
		for _, v := range vv {
			req.Headers.Add(k, v)
		}
	}

	return req
}

// Request encapsulates functionality making it easier to build REST requests.
type Request struct {
	Method  Method
	URL     *url.URL
	Headers http.Header

	prepare func(*http.Request) error
}

// HTTPRequest returns an *http.Request populated with data from r. It may be
// executed by any http.Client.
func (r *Request) HTTPRequest() (*http.Request, error) {
	req, err := http.NewRequest(string(r.Method), r.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	// merge headers
	req.Header = r.Headers

	// generate the body
	if r.prepare != nil {
		if err := r.prepare(req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// resourceURL returns a *url.URL with the path resolved for a resource under base.
func resourceURL(base *url.URL, relPath string) *url.URL {
	relPath, rawQuery := splitPathQuery(relPath)
	ref := &url.URL{Path: path.Join(base.Path, relPath), RawQuery: rawQuery}
	return base.ResolveReference(ref)
}

func splitPathQuery(relPath string) (retPath, rawQuery string) {
	parsedPath, _ := url.Parse(relPath)
	rawQuery = parsedPath.RawQuery
	retPath = strings.TrimSuffix(relPath, fmt.Sprintf("?%s", rawQuery))
	return
}

// unmarshal unmarshals a JSON object from the response object's body. If the
// Content-Type is not application/json or application/vnd* (or we can't detect
// the media type) an error is returned. The response body is always closed.
func unmarshal(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	// Don't Unmarshal Body if v is nil
	if v == nil {
		return nil
	}

	ctype, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	if !isJSONContentType(ctype) {
		return fmt.Errorf("unexpected response: %s %s", resp.Status, ctype)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// isJSONContentType returns whether or not the media type should be expected to
// contain JSON. Explicit application/json as well as types of the form
// application/vnd* and *+json are permitted. The simple checks prevent us from
// running http.DetectContentType on every request or trying to decode things
// that are clearly not JSON.
func isJSONContentType(mediaType string) bool {
	if mediaType == "application/json" ||
		strings.HasPrefix(mediaType, vndMediaTypePrefix) ||
		strings.HasSuffix(mediaType, mediaTypeJSONSuffix) {
		return true
	}
	return false
}

// RestError is returned from REST transmissions to allow for inspection of
// failed request and response contents.
type RestError struct {
	// The Request that triggered the error.
	Req *http.Request
	// The Resposne that the request returned.
	Resp *http.Response
	// err is the original error
	err error
	// ErrBody is the body of the request that errored.
	// Not named Body since there is an accessor method.
	ErrBody *string
}

func (r *RestError) Error() string {
	msg := r.err.Error()
	prefix := msg + " - "

	// Make sure the Error reads the cached body so
	// you can call error multiple times with no issues.
	// Also handle json from the endpoint and look for
	// the error field.
	if r.Body() != "" {
		type body struct {
			Error string `json:"error"`
		}

		var b *body

		jerr := json.Unmarshal([]byte(r.Body()), &b)
		if jerr != nil {
			return prefix + r.Body()
		}

		if b.Error != "" {
			return prefix + b.Error
		}

		return prefix + r.Body()
	}

	return msg
}

func (r *RestError) Body() string {
	// Return the body if we have it.
	if r.ErrBody != nil {
		return *r.ErrBody
	}

	// Easier to deal with body as regular string.
	// ErrBody is a pointer so that I can tell if it was
	// actually set to "".
	strBody := ""

	// If we don't have a body, return "".
	if r.Resp == nil || r.Resp.Body == nil {
		r.ErrBody = &strBody
		return *r.ErrBody
	}

	// Read the body, then set a new buffer
	// to the body field so the original
	// response still has a body.
	b, _ := ioutil.ReadAll(r.Resp.Body)
	defer r.Resp.Body.Close()
	buf := bytes.NewBuffer(b)
	r.Resp.Body = ioutil.NopCloser(buf)

	// Set ErrBody to the new body.
	strBody = string(b)
	r.ErrBody = &strBody

	return *r.ErrBody
}
