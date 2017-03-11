// Copyright 2012-2016 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	tt "github.com/apcera/util/testtool"
)

func TestUntarResolveDestinations(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	u := new(Untar)
	u.resolvedLinks = make([]resolvedLink, 0)

	makeTestDir(t)

	runTest := func(p, e string) {
		dst, err := u.resolveDestination(p)
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, dst, e)
	}

	runTest("a", "a")
	runTest("a/b", "a/b")
	runTest("a/b/c", "a/b/c")
	runTest("a/b/c/d", "a/b/c/d")
	runTest("a/b/c/d/e", "a/b/c/d/e")
	runTest("a/b/c/f", "a/b/c/f")
	runTest("a/b/c/l", "a/b/i")
	runTest("a/b/c/l/j", "a/b/i/j")
	runTest("a/b/c/l/j/k", "a/b/i/j/k")
	runTest("a/b/c/l/j/l", "a/b/i/j/k")
	runTest("a/b/c/l/j/m", "a/b/g")
	runTest("a/b/g", "a/b/g")
	runTest("a/b/h", "a/b/g")
	runTest("a/b/i", "a/b/i")
	runTest("a/b/i/j", "a/b/i/j")
	runTest("a/b/i/j/k", "a/b/i/j/k")
	runTest("a/b/i/j/l", "a/b/i/j/k")
	runTest("a/b/i/j/m", "a/b/g")

	// resolve an absolute path symlink relative to the root
	u.AbsoluteRoot = "/"
	runTest("a/b/bash", "/bin/bash")

	// now resolve it relative to some other arbituary path
	u.AbsoluteRoot = "/some/path/elsewhere"
	runTest("a/b/bash", "/some/path/elsewhere/bin/bash")
}

func TestUntarExtractFollowingSymlinks(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeDirectory := func(name string) {
		header := new(tar.Header)
		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode = 0755
		header.Mode |= c_ISDIR
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	writeFile := func(name, contents string) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	writeSymlink := func(name, link string) {
		header := new(tar.Header)
		header.Name = name
		header.Linkname = link
		header.Typeflag = tar.TypeSymlink
		header.Mode = 0644
		header.Mode |= c_ISLNK
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	// generate the mock tar
	writeDirectory(".")
	writeFile("./foo", "foo")
	writeDirectory("./usr")
	writeDirectory("./usr/bin")
	writeFile("./usr/bin/bash", "bash")
	writeSymlink("./usr/bin/sh", "bash")

	// now write a symlink that is an absolute path and then a file in it
	writeSymlink("./etc", "/realetc")
	writeFile("./etc/zz", "zz")
	archive.Close()

	// create temp folder to extract to
	tempDir := testHelper.TempDir()
	extractionPath := path.Join(tempDir, "pkg")
	err := os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)
	err = os.MkdirAll(path.Join(tempDir, "realetc"), 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = tempDir
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, string(b), contents)
	}

	fileSymlinks := func(name, link string) {
		l, err := os.Readlink(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, l, link)
	}

	fileExists("./pkg/foo")
	fileContents("./pkg/foo", "foo")
	fileExists("./pkg/usr")
	fileExists("./pkg/usr/bin")
	fileExists("./pkg/usr/bin/bash")
	fileContents("./pkg/usr/bin/bash", "bash")
	fileSymlinks("./pkg/usr/bin/sh", "bash")

	// now validate the symlink and file in the symlinked dir that was outside
	// the symlink should still be absolute to /realetc
	// but the file should be in ./realetc/zz within the tempDir and not the
	// system's root... so Untar follows how it knows it should resolve and not
	// follow the real symlink
	fileSymlinks("./pkg/etc", "/realetc")
	fileExists("./realetc/zz")
	fileContents("./realetc/zz", "zz")
}

func TestUntarCreatesDeeperPathsIfNotMentioned(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeFile := func(name, contents string) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	// generate the mock tar... this will write to a file in a directory that
	// isn't already created within the tar
	writeFile("./a_directory/file", "foo")
	archive.Close()

	// create temp folder to extract to
	tempDir := testHelper.TempDir()
	extractionPath := path.Join(tempDir, "pkg")
	err := os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = tempDir
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, string(b), contents)
	}

	fileExists("./pkg/a_directory/file")
	fileContents("./pkg/a_directory/file", "foo")
}

func TestUntarExtractOverwriting(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeDirectory := func(name string) {
		header := new(tar.Header)
		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode = 0755
		header.Mode |= c_ISDIR
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	writeFile := func(name, contents string) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	writeSymlink := func(name, link string) {
		header := new(tar.Header)
		header.Name = name
		header.Linkname = link
		header.Typeflag = tar.TypeSymlink
		header.Mode = 0644
		header.Mode |= c_ISLNK
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	// create temp folder to extract to
	tempDir := testHelper.TempDir()

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, string(b), contents)
	}

	fileSymlinks := func(name, link string) {
		l, err := os.Readlink(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, l, link)
	}

	// generate the mock tar
	writeDirectory(".")
	writeFile("./foo", "foo")
	writeDirectory("./usr")
	writeDirectory("./usr/bin")
	writeFile("./usr/bin/bash", "bash")
	writeSymlink("./usr/bin/sh", "bash")
	writeDirectory("./etc")
	writeFile("./etc/awesome", "awesome")
	writeFile("./var", "vvv")
	archive.Close()

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, tempDir)
	tt.TestExpectSuccess(t, u.Extract())

	// validate the first tar
	fileExists("./foo")
	fileContents("./foo", "foo")
	fileExists("./usr")
	fileExists("./usr/bin")
	fileExists("./usr/bin/bash")
	fileContents("./usr/bin/bash", "bash")
	fileSymlinks("./usr/bin/sh", "bash")
	fileExists("./etc/awesome")
	fileContents("./etc/awesome", "awesome")
	fileExists("./var")
	fileContents("./var", "vvv")

	// create another tar and then extract it
	buffer2 := bytes.NewBufferString("")
	archive = tar.NewWriter(buffer2)

	// write the 2nd tar
	writeDirectory(".")
	writeFile("./foo", "bar")
	writeDirectory("./usr")
	writeDirectory("./usr/bin")
	writeFile("./usr/bin/zsh", "zsh")
	writeSymlink("./usr/bin/sh", "zsh")
	writeFile("./etc", "etc") // replace the directory with a file
	writeDirectory("./var")   // replace the file with a directory
	writeFile("./var/lib", "lll")
	archive.Close()

	// extract the 2nd tar
	r = bytes.NewReader(buffer2.Bytes())
	u = NewUntar(r, tempDir)
	tt.TestExpectSuccess(t, u.Extract())

	// verify the contents were overwritten as expected
	fileContents("./foo", "bar")
	fileContents("./usr/bin/zsh", "zsh")
	fileSymlinks("./usr/bin/sh", "zsh")
	fileContents("./etc", "etc")
	fileContents("./var/lib", "lll")
}

func TestUntarIDMappings(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeDirectoryWithOwners := func(name string, uid, gid int) {
		header := new(tar.Header)
		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode = 0755
		header.Mode |= c_ISDIR
		header.ModTime = time.Now()
		header.Uid = uid
		header.Gid = gid
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	writeFileWithOwners := func(name, contents string, uid, gid int) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))
		header.Uid = uid
		header.Gid = gid

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	writeDirectoryWithOwners(".", 0, 0)
	writeFileWithOwners("./foo", "foo", 0, 0)
	archive.Close()

	// setup our mapping func
	usr, err := user.Current()
	tt.TestExpectSuccess(t, err)
	myUid, err := strconv.Atoi(usr.Uid)
	tt.TestExpectSuccess(t, err)
	myGid, err := strconv.Atoi(usr.Gid)
	tt.TestExpectSuccess(t, err)
	uidFuncCalled := false
	gidFuncCalled := false
	uidMappingFunc := func(uid int) (int, error) {
		uidFuncCalled = true
		tt.TestEqual(t, uid, 0)
		return myUid, nil
	}
	gidMappingFunc := func(gid int) (int, error) {
		gidFuncCalled = true
		tt.TestEqual(t, gid, 0)
		return myGid, nil
	}

	// extract
	tempDir := testHelper.TempDir()
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, tempDir)
	u.PreserveOwners = true
	u.OwnerMappingFunc = uidMappingFunc
	u.GroupMappingFunc = gidMappingFunc
	tt.TestExpectSuccess(t, u.Extract())

	// verify it was called
	tt.TestEqual(t, uidFuncCalled, true)
	tt.TestEqual(t, gidFuncCalled, true)

	// verify the file
	stat, err := os.Stat(path.Join(tempDir, "foo"))
	tt.TestExpectSuccess(t, err)
	sys := stat.Sys().(*syscall.Stat_t)
	tt.TestEqual(t, sys.Uid, uint32(myUid))
	tt.TestEqual(t, sys.Gid, uint32(myGid))
}

func TestUntarFailures(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// Bad compression type.
	u := NewUntar(strings.NewReader("bad"), "/tmp")
	u.Compression = Compression(-1)
	tt.TestExpectError(t, u.Extract())

	// FIXME(brady): add more cases here!
}

func TestCannotDetectCompression(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	u := NewUntar(strings.NewReader("bad"), "/tmp")
	u.Compression = DETECT

	tt.TestExpectError(t, u.Extract())
}

func TestUntarWhitelist(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeDirectory := func(name string) {
		header := new(tar.Header)
		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode = 0755
		header.Mode |= c_ISDIR
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	writeFile := func(name, contents string) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	writeDirectory(".")
	writeFile("./foo", "foo")
	writeFile("./foobar", "foobar")
	writeFile("./doesntexist", "foo")
	writeDirectory("./usr")
	writeDirectory("./usr/bin")
	writeFile("./usr/bin/bash", "bash")
	writeDirectory("./usr/bin/other")
	writeFile("./usr/bin/other/sh", "sh")
	writeDirectory("./usr/nope")
	writeFile("./usr/nope/not", "notthere")
	writeDirectory("./usr/justdir")
	writeFile("./usr/justdir/not", "notthere")
	writeDirectory("./etc")
	writeFile("./etc/not", "notthere")

	archive.Close()

	// create temp folder to extract to
	tempDir := testHelper.TempDir()

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, tempDir)
	u.PathWhitelist = []string{
		"/foo",
		"/usr/bin/",
		"/usr/justdir",
	}
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
	}

	fileNotExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectError(t, err)
	}

	fileExists("/foo")
	fileExists("/usr/bin/bash")
	fileExists("/usr/bin/other/sh")
	fileExists("/usr/justdir")
	fileNotExists("/foobar")
	fileNotExists("/doesntexist")
	fileNotExists("/usr/nope/not")
	fileNotExists("/usr/justdir/not")
	fileNotExists("/etc/not")
}

func TestUntarCustomHandler(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// create a buffer and tar.Writer
	buffer := bytes.NewBufferString("")
	archive := tar.NewWriter(buffer)

	writeDirectory := func(name string) {
		header := new(tar.Header)
		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode = 0755
		header.Mode |= c_ISDIR
		header.ModTime = time.Now()
		tt.TestExpectSuccess(t, archive.WriteHeader(header))
	}

	writeFile := func(name, contents string) {
		b := []byte(contents)
		header := new(tar.Header)
		header.Name = name
		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Mode |= c_ISREG
		header.ModTime = time.Now()
		header.Size = int64(len(b))

		tt.TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		tt.TestExpectSuccess(t, err)
		tt.TestExpectSuccess(t, archive.Flush())
	}

	writeDirectory(".")
	writeFile("./foo", "foo")
	writeFile("./foobar", "foobar")

	archive.Close()

	// create temp folder to extract to
	tempDir := testHelper.TempDir()

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, tempDir)
	u.CustomHandlers = []UntarCustomHandler{
		func(rootpath string, header *tar.Header, reader io.Reader) (bool, error) {
			if filepath.Clean(header.Name) != "foobar" {
				return false, nil
			}

			f, err := os.Create(filepath.Join(rootpath, "foobar2"))
			if err != nil {
				return false, err
			}
			defer f.Close()
			if _, err := io.Copy(f, reader); err != nil {
				return false, err
			}
			return true, nil
		},
	}
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectSuccess(t, err)
	}

	fileNotExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		tt.TestExpectError(t, err)
	}

	fileExists("/foo")
	fileNotExists("/foobar")
	fileExists("/foobar2")
}
