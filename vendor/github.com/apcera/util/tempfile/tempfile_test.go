// Copyright 2015 Apcera Inc. All rights reserved.

package tempfile

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestTempFile(t *testing.T) {
	r := strings.NewReader("blah")
	f, err := New(r)
	tt.TestExpectSuccess(t, err)

	b, err := ioutil.ReadAll(f)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(b), "blah")

	tf := f.(*unlinkOnCloseFile)
	st, err := os.Stat(tf.File.Name())
	tt.TestExpectSuccess(t, err)
	tt.TestNotEqual(t, st, nil)

	tt.TestExpectSuccess(t, f.Close())

	_, err = os.Stat(tf.File.Name())
	tt.TestExpectError(t, err)
	tt.TestEqual(t, os.IsNotExist(err), true)
}
