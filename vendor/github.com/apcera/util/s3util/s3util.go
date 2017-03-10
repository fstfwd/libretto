// Copyright 2014 Apcera, Inc. All rights reserved.

package s3util

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/apcera/util/hmac"
)

const (
	// S3_URL is the URL that S3 is located at.
	S3_URL = "s3.amazonaws.com"

	scheme = "https"
)

// An S3Uploader is a helper object for uploading S3 requests.
type S3Uploader struct {
	// s3url is the parsed URL of the s3 account we're uploading to.
	s3url *url.URL

	// bucketName is the name of the specific bucket we're uploading to.
	bucketName string

	// permission is the permission we're uploading with.
	permission string

	// out is where output is written.
	out io.Writer
}

// NewS3Uploader configures a new S3 uploader from a uri.
func NewS3Uploader(bucket, permission string, quiet bool) (*S3Uploader, error) {
	u := &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s.%s", bucket, S3_URL),
	}

	permission = strings.ToLower(permission)
	if err := validatePermission(permission); err != nil {
		return nil, err
	}

	uploader := &S3Uploader{
		s3url:      u,
		bucketName: bucket,
		permission: permission,
		out:        os.Stdout,
	}
	if quiet {
		uploader.out = ioutil.Discard
	}
	return uploader, nil
}

// validatePermission enforces that a valid permission has been supplied.
// ACL documentation: http://docs.aws.amazon.com/AmazonS3/latest/API/RESTObjectPUTacl.html
func validatePermission(permission string) error {
	switch permission {
	case "public-read":
		return nil
	case "private":
		return nil
	case "public-read-write":
		return nil
	case "authenticated-read":
		return nil
	case "bucket-owner-read":
		return nil
	case "bucket-owner-full-control":
		return nil
	}
	return fmt.Errorf("unsupported permission: %q", permission)
}

// UploadToS3 prepares a buffer for upload to S3.
func (s *S3Uploader) UploadToS3(fPath string, buf *bytes.Buffer) error {
	fmt.Fprintf(s.out, "Preparing %q for upload to s3 bucket %q...", filepath.Base(fPath), s.s3url.String())

	req, err := s.buildS3Request(filepath.Base(fPath), buf)
	if err != nil {
		fmt.Fprintln(s.out, " error")
		return err
	}
	fmt.Fprintln(s.out, " done")

	fmt.Fprint(s.out, "Uploading...")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(s.out, " error")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(s.out, " error")
		if resp.StatusCode == 403 {
			fmt.Fprintln(s.out, "Encountered authorization error! Are your keys set correctly?")
		}
		errMsg := fmt.Sprintf("received code %d, should have received %d", resp.StatusCode, http.StatusOK)
		b, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			errMsg = fmt.Sprintf("%s\n\nReceived S3 Response: %q", errMsg, string(b))
		}
		return fmt.Errorf(errMsg)
	}
	fmt.Fprintln(s.out, " done")
	return nil
}

// buildS3Request constructs an http request for the upload
func (s *S3Uploader) buildS3Request(fileBase string, buffer *bytes.Buffer) (*http.Request, error) {
	u := &url.URL{
		Scheme: s.s3url.Scheme,
		Host:   s.s3url.Host,
		Path:   fileBase,
	}
	// This is a PUT request containing the data buffer.
	req, err := http.NewRequest("PUT", u.String(), buffer)
	if err != nil {
		return nil, err
	}

	// FIXME: Send 'Connection: close' header on HTTP requests
	// as reusing the connection is still prone to EOF/Connection reset errors
	// in certain network environments...
	// See: ENGT-9670
	// https://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors
	req.Close = true

	// Generate MD5 to ensure reliable delivery
	h := md5.New()
	if _, err := h.Write(buffer.Bytes()); err != nil {
		return nil, err
	}
	md5 := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Get current time in UTC format for Date header.
	curTime := time.Now().UTC().Format(time.RFC1123)
	req.Header.Add("Date", curTime)
	req.Header.Add("Content-Md5", md5)
	req.Header.Add("Content-Type", "application/x-gzip")
	req.Header.Add("X-Amz-Acl", s.permission)

	authHeader, err := s.buildAuthHeader(md5, curTime, fileBase)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", authHeader)

	// Set content length so we avoid chunked requests.
	req.ContentLength = int64(buffer.Len())
	return req, nil
}

// buildAuthHeader builds an authorization header for S3.
func (s *S3Uploader) buildAuthHeader(md5, headerTime, fileName string) (string, error) {
	access, err := getAuthCredentials()
	if err != nil {
		return "", err
	}

	// Docs: http://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html
	verStr := fmt.Sprintf("PUT\n%s\napplication/x-gzip\n%s\nx-amz-acl:%s\n/%s", md5, headerTime, s.permission,
		path.Join(s.bucketName, fileName))
	encodedStr := hmac.ComputeHmacSha1(verStr, access.SecretKey)
	return fmt.Sprintf("AWS %s:%s", access.AccessKey, encodedStr), nil
}

// An awsAccess is a helper struct for organizing AWS credentials. Fields are
// exported for json marshalling. (currently not implemented)
type awsAccess struct {
	AccessKey string `json:"aws_access_key_id"`
	SecretKey string `json:"aws_secret_key"`
}

// These constants correspond to the expected credential-containing environment
// variable keys.
const (
	AWS_ACCESS_KEY_ID = "AWS_ACCESS_KEY_ID"
	AWS_SECRET_KEY    = "AWS_SECRET_KEY"
)

// getAuthCredentials retrieves authorization credentials from configuration.
// TODO(alex): write to/retrieve from disk json file.
func getAuthCredentials() (*awsAccess, error) {
	awsAccessKeyId := os.Getenv(AWS_ACCESS_KEY_ID)
	if awsAccessKeyId == "" {
		return nil, fmt.Errorf("could not get AWS access key")
	}
	awsSecretKey := os.Getenv(AWS_SECRET_KEY)
	if awsSecretKey == "" {
		return nil, fmt.Errorf("could not get AWS secret key")
	}
	return &awsAccess{
		AccessKey: awsAccessKeyId,
		SecretKey: awsSecretKey,
	}, nil
}
