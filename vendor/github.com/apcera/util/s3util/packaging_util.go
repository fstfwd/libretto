// Copyright 2014 Apcera, Inc. All rights reserved.

package s3util

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Zipper zips file contents at a path. Helper to prepare zipped data for upload
// to S3.
func Zipper(zipPath string) (*bytes.Buffer, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buffer := bytes.NewBuffer(nil)
	w := zip.NewWriter(buffer)
	fh := zip.FileHeader{}
	fh.Name = filepath.Base(zipPath)
	fh.SetMode(0755)

	// Add some files to the archive.
	var files = []struct {
		Name       string
		FileHandle *os.File
		Header     *zip.FileHeader
	}{
		{fh.Name, f, &fh},
	}

	// Loop through all files listed above
	// Can be created using file.Name or a custom file.Header for flexibility.
	// To set permissions you need to use the file.Header construct.
	for _, file := range files {
		var zf io.Writer
		if file.Header == nil {
			zf, err = w.Create(file.Name)
		} else {
			zf, err = w.CreateHeader(file.Header)
		}
		if err != nil {
			return nil, err
		}

		if _, err := io.Copy(zf, f); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buffer, nil
}

// Gzipper gzips the file at the path. Helper to prepare gzipped data for upload
// to S3.
func Gzipper(gzipPath string) (*bytes.Buffer, error) {
	f, err := os.Open(gzipPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Gzip the binary data and toss it in a buffer.
	buffer := bytes.NewBuffer(nil)
	gzipData := gzip.NewWriter(buffer)
	gzipData.Header.Name = filepath.Base(gzipPath)
	gzipData.Header.ModTime = time.Now()
	if _, err := io.Copy(gzipData, f); err != nil {
		return nil, err
	}
	if err := gzipData.Close(); err != nil {
		return nil, err
	}
	return buffer, nil
}
