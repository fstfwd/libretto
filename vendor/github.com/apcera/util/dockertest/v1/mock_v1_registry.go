// Copyright 2014-2015 Apcera Inc. All rights reserved.

package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

var (
	testVerbose    = false // Change to true in order to see HTTP requests in test output.
	testHttpServer *httptest.Server
	testLayers     = map[string]map[string]string{
		"badcafe": {
			"json":     `{"id":"badcafe","k1": "v1"}`,
			"ancestry": `["badcafe"]`,
			"layer":    string([]byte{0xa1, 0xb2, 0xc3}),
			"checksum": "1ac330d",
		},
		"deadbeef": {
			"json":     `{"id":"deadbeef","k2": "v2"}`,
			"ancestry": `["deadbeef", "badcafe"]`,
			"layer":    string([]byte{0xd4, 0xe5, 0xf6}),
			"checksum": "2bd330f",
		},
		"bd51c4e1b5aceec2ff4bdd87d3fe5f1f16e1120490dee47e2999036f5bc55ccf": { // A random (valid) LayerID
			"json":     `{"id":"bd51c4e1b5aceec2ff4bdd87d3fe5f1f16e1120490dee47e2999036f5bc55ccf","k1": "v1"}`,
			"ancestry": `["bd51c4e1b5aceec2ff4bdd87d3fe5f1f16e1120490dee47e2999036f5bc55ccf"]`,
			"layer":    string([]byte{0xa1, 0xb2, 0xc3}),
			"checksum": "abcd1234",
		},
	}
	testRepositories = map[string]map[string]string{
		"foo/bar": {
			"latest": "deadbeef",
			"base":   "badcafe",
		},
		"some/image": {
			"latest": "bd51c4e1b5aceec2ff4bdd87d3fe5f1f16e1120490dee47e2999036f5bc55ccf",
		},
		"base": {
			"latest": "badcafe",
		},
	}
	mu sync.Mutex
)

func RunMockRegistry() *httptest.Server {
	mu.Lock()
	defer mu.Unlock()

	if testHttpServer != nil {
		return testHttpServer
	}

	r := mux.NewRouter()
	r.HandleFunc("/v1/images/{image_id:[^/]+}/{data_type:json|layer|ancestry}", handlerImage).Methods("GET")
	r.HandleFunc("/v1/repositories/{repository:.+}/tags", handlerTags).Methods("GET")
	r.HandleFunc("/v1/repositories/{repository:.+}/images", handlerImages).Methods("GET")

	testHttpServer = httptest.NewServer(logHandler(r))
	return testHttpServer
}

func logHandler(handler http.Handler) http.Handler {
	if !testVerbose {
		return handler
	}
	lh := func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s \"%s %s\"\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(lh)
}

func writeResponse(w http.ResponseWriter, httpStatus int, payload interface{}) {
	w.WriteHeader(httpStatus)
	body, err := json.Marshal(payload)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	w.Write(body)
}

func checkAuth(w http.ResponseWriter, r *http.Request) bool {
	writeAuth := func() {
		sessionID := fmt.Sprintf("FAKE-SESSION-%d", time.Now().UnixNano())
		token := fmt.Sprintf("FAKE-TOKEN-%d", time.Now().UnixNano())

		http.SetCookie(w, &http.Cookie{Name: "session", Value: sessionID, MaxAge: 3600})
		w.Header().Add("X-Docker-Token", token)
	}

	// Mock registry doesn't really perform any auth.
	if len(r.Cookies()) > 0 {
		writeAuth()
		return true
	}

	if (len(r.Header.Get("Authorization"))) > 0 {
		writeAuth()
		return true
	}
	w.Header().Add("WWW-Authenticate", "token")
	writeResponse(w, 401, "Bad auth")
	return false
}

func handlerImages(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(testHttpServer.URL) // Ignoring error, URL is guaranteed to be OK.

	w.Header().Add("X-Docker-Endpoints", fmt.Sprintf("%s     ,    %s", u.Host, "test.example.com"))
	w.Header().Add("X-Docker-Token", fmt.Sprintf("FAKE-SESSION-%d", time.Now().UnixNano()))

	var images []map[string]string

	for imageID, layer := range testLayers {
		image := make(map[string]string)
		image["id"] = imageID
		image["checksum"] = layer["checksum"]
		images = append(images, image)
	}
	writeResponse(w, 200, images)
}

func handlerImage(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	layer, exists := testLayers[vars["image_id"]]
	if !exists {
		http.NotFound(w, r)
		return
	}

	layer_size := len(layer["layer"])
	w.Header().Add("X-Docker-Size", strconv.Itoa(layer_size))
	io.WriteString(w, layer[vars["data_type"]])
}

func handlerTags(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	tags, exists := testRepositories[vars["repository"]]
	if !exists {
		http.NotFound(w, r)
		return
	}

	writeResponse(w, 200, tags)
}
