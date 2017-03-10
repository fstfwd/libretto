// Copyright 2015 Apcera Inc. All rights reserved.

package tempfile

import (
	"io"
	"io/ioutil"
	"os"
)

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type unlinkOnCloseFile struct {
	*os.File
}

func (f *unlinkOnCloseFile) Close() error {
	cerr := f.File.Close()
	rerr := os.Remove(f.Name())
	if cerr != nil {
		return cerr
	}
	return rerr
}

// New will take in a io.Reader and write its content to a new temporary
// file. The temporary file will be automatically removed when it closed.
func New(r io.Reader) (ReadSeekCloser, error) {
	tf, err := ioutil.TempFile(os.TempDir(), "temporary-file")
	if err != nil {
		return nil, err
	}

	f := &unlinkOnCloseFile{tf}
	successful := false
	defer func() {
		if !successful {
			f.Close()
		}
	}()

	if _, err := io.Copy(f, r); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	successful = true
	return f, nil
}
