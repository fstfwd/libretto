// Copyright 2014 Apcera, Inc. All rights reserved.

package s3util

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNewS3Uploader(t *testing.T) {
	bucket := "test-uploads"
	permission := "public-read"
	uploader, err := NewS3Uploader(bucket, permission, true)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if uploader == nil {
		t.Fatal("Uploader should not be nil.")
	}
	if uploader.s3url.Host != fmt.Sprintf("%s.%s", bucket, S3_URL) {
		t.Fatalf("Expected s3Url %q host to contain bucket %q", uploader.s3url.Host, bucket)
	}

	uploader, err = NewS3Uploader(bucket, "invalid-acl-permission", true)
	if err == nil {
		t.Fatal("Expected invalid permission error")
	}
}

func TestZipper(t *testing.T) {
	dir := filepath.Join(os.TempDir(), strconv.Itoa(rand.Int()))
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("Should have been able to make tempdir: %s", err)
	}
	defer os.Remove(dir)

	fName := "tempfile"
	fullName := filepath.Join(dir, fName)
	data := []byte("\ntest data\n")
	if err := ioutil.WriteFile(fullName, data, 0644); err != nil {
		t.Fatalf("Should have been able to write file: %s", err)
	}
	defer os.Remove(fullName)
	buf, err := Zipper(fullName)
	if err != nil {
		t.Fatalf("Should have been able to zip data: %s", err)
	}
	if len(buf.Bytes()) == 0 {
		t.Fatalf("Expected nonzero length from buffer: %v", buf)
	}
	if !bytes.Contains(buf.Bytes(), []byte(fName)) {
		t.Fatalf("Expected buf %q to contain file name %q", string(buf.Bytes()), fName)
	}
	if !bytes.Contains(buf.Bytes(), data) {
		t.Fatalf("Expected buf %q to contain data %q", string(buf.Bytes()), string(data))
	}
}

func TestGZipper(t *testing.T) {
	dir := filepath.Join(os.TempDir(), strconv.Itoa(rand.Int()))
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("Should have been able to make tempdir: %s", err)
	}
	defer os.Remove(dir)

	fName := "tempfile"
	fullName := filepath.Join(dir, fName)
	data := []byte("\ntest data\n")
	if err := ioutil.WriteFile(fullName, data, 0644); err != nil {
		t.Fatalf("Should have been able to write file: %s", err)
	}
	defer os.Remove(fullName)
	buf, err := Gzipper(fullName)
	if err != nil {
		t.Fatalf("Should have been able to gzip data: %s", err)
	}
	if len(buf.Bytes()) == 0 {
		t.Fatalf("Expected nonzero length from buffer: %v", buf)
	}
	if !bytes.Contains(buf.Bytes(), []byte(fName)) {
		t.Fatalf("Expected buf %q to contain file name %q", string(buf.Bytes()), fName)
	}

	reader, err := gzip.NewReader(buf)
	if err != nil {
		t.Fatalf("Error creating gzip reader: %s", err)
	}
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatalf("Error reading gzip reader: %s", err)
	}
	if !bytes.Contains(b, data) {
		t.Fatalf("Expected buf %q to contain data %q", string(b), string(data))
	}
}

func TestBuildS3Request(t *testing.T) {
	bucket := "test-uploads"
	permission := "public-read"
	uploader, err := NewS3Uploader(bucket, permission, true)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := os.Setenv(AWS_SECRET_KEY, "foo"); err != nil {
		t.Fatalf("Error setting environment: %s", err)
	}
	if err := os.Setenv(AWS_ACCESS_KEY_ID, "foo"); err != nil {
		t.Fatalf("Error setting environment: %s", err)
	}

	fileBase := "foo"
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("some data")
	req, err := uploader.buildS3Request(fileBase, buffer)
	if err != nil {
		t.Fatalf("Unexpected error building request: %s", err)
	}
	if req.Method != "PUT" {
		t.Fatalf("Expected request method to be 'PUT'; got %q", req.Method)
	}
	if !strings.Contains(req.URL.Host, S3_URL) {
		t.Fatalf("Expected request URL (%s) to contain S3 URI", req.URL.String())
	}
	if !strings.Contains(req.URL.String(), bucket) {
		t.Fatalf("Expected request URL (%s) to contain bucket", req.URL.String())
	}
	if req.Body != ioutil.NopCloser(buffer) {
		t.Fatal("Expected request body to equal input buffer")
	}
	if req.ContentLength != int64(9) {
		t.Fatalf("Expected content length of 9, got %d", req.ContentLength)
	}
	if req.Header["Content-Type"][0] != "application/x-gzip" {
		t.Fatalf("Expected content type of request to be 'application/x-gzip'; got %s", req.Header["Content-Type"][0])
	}
	if req.Header["X-Amz-Acl"][0] != permission {
		t.Fatalf("Expected 'X-Amz-Acl' header contents to be %s; got %s", permission, req.Header["X-Amz-Acl"][0])
	}

	md5 := req.Header["Content-Md5"][0]
	date := req.Header["Date"][0]
	authHeader, err := uploader.buildAuthHeader(md5, date, fileBase)
	if err != nil {
		t.Fatalf("Encountered error build auth header: %s", err)
	}
	if md5 != "HlAhCgICSX+3m8OLat5sNA==" {
		t.Fatalf("Expected md5 of 'HlAhCgICSX+3m8OLat5sNA==', got %q", md5)
	}
	if !strings.Contains(authHeader, fileBase) {
		t.Fatalf("Expected authHeader (%s) to contain fileBase %s", authHeader, fileBase)
	}
}

func TestUploadToS3(t *testing.T) {
	bucket := "test-uploads"
	permission := "public-read"
	uploader, err := NewS3Uploader(bucket, permission, true)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	dir := filepath.Join(os.TempDir(), strconv.Itoa(rand.Int()))
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("Should have been able to make tempdir: %s", err)
	}
	defer os.Remove(dir)

	fName := "tempfile"
	fullName := filepath.Join(dir, fName)
	data := []byte("\ntest data\n")
	if err := ioutil.WriteFile(fullName, data, 0644); err != nil {
		t.Fatalf("Should have been able to write file: %s", err)
	}
	defer os.Remove(fullName)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	newUrl, err := url.Parse("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	uploader.s3url = newUrl

	buf, err := Zipper(fullName)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	sMux := http.NewServeMux()
	sMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := http.StatusOK
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-gzip" {
			code = http.StatusBadRequest
		}
		ah := r.Header.Get("Authorization")
		if ah == "" {
			code = http.StatusUnauthorized
		}
		b, _ := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if !bytes.Contains(b, data) {
			code = http.StatusBadRequest
		}
		w.WriteHeader(code)
	})

	ts := httptest.NewUnstartedServer(sMux)
	ts.Listener = listener
	ts.Start()
	defer ts.Close()

	if err := uploader.UploadToS3(fullName, buf); err != nil {
		t.Fatalf("Error posting request: %s", err)
	}
}
