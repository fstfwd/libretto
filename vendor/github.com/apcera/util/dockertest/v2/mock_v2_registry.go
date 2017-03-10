// Copyright 2015 Apcera Inc. All rights reserved.

package v2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gorilla/mux"
)

var (
	testVerbose    = false // Change to true in order to see HTTP requests in test output.
	testHttpServer *httptest.Server
	mu             sync.Mutex

	// skipAuth skips sending authorization challenges entirely.
	skipAuth bool

	// Note: currently does not support supplying signed manifests.
	testImageManifests = map[string]string{
		"library/nats:latest":   libraryNatsLatestManifest,
		"library/foobar:latest": libraryFoobarLatestManifest,
	}
)

func RunMockRegistry() *httptest.Server {
	mu.Lock()
	defer mu.Unlock()

	if testHttpServer != nil {
		return testHttpServer
	}

	r := mux.NewRouter()

	r.HandleFunc("/token", handlerToken).Methods("GET")
	r.HandleFunc("/v2/", handlerSupport).Methods("GET")
	r.HandleFunc("/v2/{repo:[^/]+}/{image_name:[^/]+}/manifests/{image_ref:[^/]+}", handlerImageManifest).Methods("GET")
	r.HandleFunc("/v2/{repo:[^/]+}/{image_name:[^/]+}/blobs/{blob_ref:[^/]+}", handlerBlob).Methods("GET")

	testHttpServer = httptest.NewServer(logHandler(r))
	return testHttpServer
}

// SetSkipAuth allows for configuring the mock registry to not send auth
// challenges.
func SetSkipAuth(enabled bool) {
	mu.Lock()
	defer mu.Unlock()
	skipAuth = enabled
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
	header := w.Header()
	header.Add("Docker-Distribution-API-Version", "registry/2.0")

	if skipAuth {
		return true
	}

	if (len(r.Header.Get("Authorization"))) > 0 {
		return true
	}

	realm := fmt.Sprintf("http://%s/token", r.Host)
	service := fmt.Sprintf("http://%s", r.Host)

	// TODO: allow user to specify a scope on the request and have it respected?
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, realm, service))
	writeResponse(w, http.StatusUnauthorized, "Bad auth")
	return false
}

func handlerToken(w http.ResponseWriter, r *http.Request) {
	// Token request requires service and scope; see:
	//
	// https://docs.docker.com/registry/spec/auth/token/
	//
	// Expected form: `service=registry.docker.io&scope=repository:library/nats"`
	query := r.URL.Query()

	service := query.Get("service")
	scope := query.Get("scope")

	if service == "" {
		writeResponse(w, http.StatusBadRequest, "bad request: service is required")
	} else if scope == "" {
		writeResponse(w, http.StatusBadRequest, "bad request: scope is required")
	}

	tokenResponse := `{"token": "Bearer someBearerToken"}`

	io.WriteString(w, tokenResponse)
}

func handlerSupport(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	w.Write([]byte("v2 API supported!"))
}

func handlerImageManifest(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	repo, exists := vars["repo"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	imageName, exists := vars["image_name"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Tag or digest.
	imageRef, exists := vars["image_ref"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	manifest, exists := testImageManifests[fmt.Sprintf("%s/%s:%s", repo, imageName, imageRef)]
	if !exists {
		http.NotFound(w, r)
		return
	}
	io.WriteString(w, manifest)
}

func handlerBlob(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	_, exists := vars["repo"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	_, exists = vars["image_name"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	blobRef, exists := vars["blob_ref"]
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Just write back the blob reference; completely fake content, not even a tar.
	io.WriteString(w, blobRef)
}
