// Copyright 2012-2014 Apcera Inc. All rights reserved.

package restclient

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	tt "github.com/apcera/util/testtool"
)

type person struct {
	Name string
	Age  int
}

func TestResourceURL(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	base := "http://example.com:8080/v1/resources"
	baseURL, err := url.Parse(base)
	tt.TestExpectSuccess(t, err)

	examples := []struct{ in, out string }{
		// Returns base when path == ""
		{"", "http://example.com:8080/v1/resources"},
		// Adds relative paths to end
		{"items/1234", "http://example.com:8080/v1/resources/items/1234"},
		{"/items/1234", "http://example.com:8080/v1/resources/items/1234"},
	}

	for i, ex := range examples {
		u := resourceURL(baseURL, ex.in)
		if u.String() != ex.out {
			t.Errorf("%d. resourceURL(..., %q) resolved incorrectly.\nhave: %s\nwant: %s",
				i, ex.in, u.String(), ex.out)
		}
	}
}

func TestHelper_newRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	client, err := New("http://example.com/resources")
	tt.TestExpectSuccess(t, err)
	req := client.newRequest(GET, "/foos")

	tt.TestEqual(t, req.Method, GET)
	tt.TestEqual(t, req.URL.String(), "http://example.com/resources/foos")
	tt.TestEqual(t, req.Headers, http.Header(map[string][]string{}))
}

func TestHelper_newRequestWithParams(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	client, err := New("http://example.com/resources")
	tt.TestExpectSuccess(t, err)
	req := client.newRequest(GET, "/excellence?yeah=stuff&more=params")

	tt.TestEqual(t, req.Method, GET)
	tt.TestEqual(t, req.URL.String(), "http://example.com/resources/excellence?yeah=stuff&more=params")
	tt.TestEqual(t, req.Headers, http.Header(map[string][]string{}))
}

func TestNewRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	method, path, body := "", "", ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		method = req.Method
		path = req.URL.Path

		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		body = string(b)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewRequest("POST", "/blobs", "text/plain", strings.NewReader("I am a giant blob of bytes!"))
	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)

	// Verify request as received by server
	tt.TestEqual(t, method, "POST")
	tt.TestEqual(t, path, "/blobs")
	tt.TestEqual(t, body, "I am a giant blob of bytes!")
}

func TestNewRequestHeaders(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	headerValue := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		headerValue = req.Header.Get("Sample")
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	client.Headers.Set("Sample", "applesauce")
	req := client.NewRequest("POST", "/blobs", "text/plain", strings.NewReader("I am a giant blob of bytes!"))
	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)

	// Verify request to the server had the correct header value
	tt.TestEqual(t, headerValue, "applesauce")
}

func TestBasicJsonRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	method, path, body := "", "", ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		method = req.Method
		path = req.URL.Path

		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"foo":"bar"}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/foo", map[string]string{"bar": "baz"})

	var res map[string]string
	err = client.Result(req, &res)
	tt.TestExpectSuccess(t, err)

	// Verify request as received by server
	tt.TestEqual(t, method, "POST")
	tt.TestEqual(t, path, "/foo")
	tt.TestEqual(t, body, `{"bar":"baz"}`+"\n")

	// Verify response was parsed by client
	tt.TestEqual(t, len(res), 1)
	tt.TestEqual(t, res["foo"], "bar")
}

func TestJsonStructRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	var receivedPerson *person

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&receivedPerson)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"Name":"Molly","Age":45}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/", person{Name: "John", Age: 56})

	var responsePerson person
	err = client.Result(req, &responsePerson)
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, receivedPerson.Name, "John")
	tt.TestEqual(t, receivedPerson.Age, 56)

	tt.TestEqual(t, responsePerson.Name, "Molly")
	tt.TestEqual(t, responsePerson.Age, 45)
}

func TestFormRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		form = req.Form
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"foo":"bar"}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("POST", "/", map[string]string{"name": "Tim"})
	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)

	// Verify form data as received by server
	tt.TestEqual(t, form.Get("name"), "Tim")
}

func TestErrorResult(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		io.WriteString(w, "Didn't work")
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	err = client.Result(req, nil)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "error in response: 500 Internal Server Error - Didn't work")
	tt.TestEqual(t, rerr.Body(), "Didn't work")

	rerr2 := new(RestError)
	rerr2.err = fmt.Errorf("foo bar baz wibble")
	tt.TestEqual(t, rerr2.Error(), "foo bar baz wibble")

	rerr2 = new(RestError)
	rerr2.Resp = &http.Response{
		StatusCode: 404,
	}
	rerr2.err = fmt.Errorf("foo bar baz wibble")
	tt.TestEqual(t, rerr2.Error(), "foo bar baz wibble")
}

func TestErrorResponse(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		io.WriteString(w, "Didn't work")
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	resp, err := client.Do(req)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "error in response: 500 Internal Server Error - Didn't work")
	tt.TestEqual(t, rerr.Body(), "Didn't work")

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(body), "Didn't work")
}

func TestErrorResponseWithJson(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		io.WriteString(w, `{"error": "some error"}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	resp, err := client.Do(req)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "error in response: 500 Internal Server Error - some error")
	tt.TestEqual(t, rerr.Body(), `{"error": "some error"}`)

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(body), `{"error": "some error"}`)
}

func TestErrorResponseNoBody(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	resp, err := client.Do(req)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "error in response: 500 Internal Server Error")

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(body), "")
}

func TestInvalidJsonResponse(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `"Name":"Molly","Age":45}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("GET", "/", nil)

	var responsePerson person
	err = client.Result(req, &responsePerson)
	tt.TestExpectError(t, err)
	tt.TestNotEqual(t, err.(*json.UnmarshalTypeError), nil, "Should have been a json unmarshal error")
}

// TestParseMimetype ensures that client.Result handles the expected media types
// correctly.
func TestParseMimetype(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	testCases := []struct {
		desc           string
		handler        http.Handler
		expectedPerson person
		expectedErr    error
	}{
		{
			desc: "Response with application/json should work",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(200)
				io.WriteString(w, `{"Name":"Molly","Age":45}`)
			}),
			expectedPerson: person{Name: "Molly", Age: 45},
		},
		{
			desc: "Response with application/vnd type should work",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.Foo; charset=utf-8")
				w.WriteHeader(200)
				io.WriteString(w, `{"Name":"Molly","Age":45}`)
			}),
			expectedPerson: person{Name: "Molly", Age: 45},
		},
		{
			desc: "Response with +json suffix should work",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/foo+json; charset=utf-8")
				w.WriteHeader(200)
				io.WriteString(w, `{"Name":"Molly","Age":45}`)
			}),
			expectedPerson: person{Name: "Molly", Age: 45},
		},
		{
			desc: "Response with any other type should error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "foo")
				w.WriteHeader(200)
				io.WriteString(w, `{"Name":"Molly","Age":45}`)
			}),
			expectedErr: fmt.Errorf("unexpected response: 200 OK foo"),
		},
	}

	for _, tc := range testCases {
		// Create test server; defer close will leave them open for the duration
		// longer than one loop but will ensure they all get shutdown even when
		// testing fails.
		server := httptest.NewServer(tc.handler)
		defer server.Close()
		client, err := New(server.URL)
		tt.TestExpectSuccess(t, err)

		req := client.NewJsonRequest("GET", "/", nil)
		var responsePerson person
		err = client.Result(req, &responsePerson)
		tt.TestEqual(t, err, tc.expectedErr, tc.desc)
		tt.TestEqual(t, responsePerson, tc.expectedPerson, tc.desc)
	}
}

func TestEmptyPostRequest(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a test server
	body := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		body = string(b)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/", nil)

	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, body, "")
}
