// Copyright 2013-2016 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	tt "github.com/apcera/util/testtool"
)

func makeTestDir(t *testing.T) string {
	testHelper := tt.StartTest(t)
	//defer testHelper.FinishTest()

	cwd, err := os.Getwd()
	tt.TestExpectSuccess(t, err)
	testHelper.AddTestFinalizer(func() {
		tt.TestExpectSuccess(t, os.Chdir(cwd))
	})
	dir := testHelper.TempDir()
	tt.TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	os.Mkdir(cwd, mode) //Don't care about return value.  For some reason CWD is not created by go test on all systems.
	tt.TestExpectSuccess(t, os.Mkdir("a", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i/j", mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/c/d/e", []byte{}, mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/c/f", []byte{}, mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/g", []byte{}, mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j/k", []byte{}, mode))
	tt.TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	tt.TestExpectSuccess(t, os.Symlink("../i", "a/b/c/l"))
	tt.TestExpectSuccess(t, os.Symlink("g", "a/b/h"))
	tt.TestExpectSuccess(t, os.Symlink("k", "a/b/i/j/l"))
	tt.TestExpectSuccess(t, os.Symlink("../../g", "a/b/i/j/m"))
	return dir
}

func TestTarSimple(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tt.TestExpectSuccess(t, tw.Archive())
}

func TestTarVirtualPath(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.VirtualPath = "foo"
	tt.TestExpectSuccess(t, tw.Archive())
}

func TestExcludeRootPath(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.ExcludeRootPath = true
	tt.TestEqual(t, tw.excludeRootPath("./"), true)
	tt.TestExpectSuccess(t, tw.Archive())

	archive := tar.NewReader(w)
	rootHeader := ""
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if header.Name == "./" {
			rootHeader = header.Name
		}
	}

	tt.TestNotEqual(t, rootHeader, "./")
}

func TestIncludeRootPath(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.ExcludeRootPath = false
	tt.TestEqual(t, tw.excludeRootPath("./"), false)
	tt.TestExpectSuccess(t, tw.Archive())

	archive := tar.NewReader(w)
	rootHeader := ""
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if header.Name == "./" {
			rootHeader = header.Name
		}
	}

	tt.TestEqual(t, rootHeader, "./")
}

func TestPathExclusion(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	type testcase struct {
		RE       string // e.g. "p.*h"
		Path     string // e.g. "path"
		Expected map[string]bool
	}

	testcases := []testcase{
		{
			RE: "simple", Path: "simple",
			Expected: map[string]bool{
				"simple":                      true,
				"/simple":                     true,
				"simple/":                     true,
				"/simple/":                    true,
				"/before/simple":              true,
				"/three/levels/before/simple": true,
			},
		}, {
			RE: "/simple", Path: "simple",
			Expected: map[string]bool{
				"/simple": true, "/simple/": true,
			},
		}, {
			RE:       "slash/",
			Path:     "slash",
			Expected: map[string]bool{},
		}, {
			RE:       "/simple/",
			Path:     "simple",
			Expected: map[string]bool{},
		}, {
			RE:   "sim.*-RE",
			Path: "simple-RE",
			Expected: map[string]bool{
				"simple-RE":                      true,
				"/simple-RE":                     true,
				"simple-RE/":                     true,
				"/simple-RE/":                    true,
				"/before/simple-RE":              true,
				"/three/levels/before/simple-RE": true,
				"simp-middle-le-RE":              true,
			},
		}, {
			RE:   "simple-RE.*",
			Path: "simple-RE",
			Expected: map[string]bool{
				"simple-RE":                      true,
				"/simple-RE":                     true,
				"simple-RE/":                     true,
				"/simple-RE/":                    true,
				"/before/simple-RE":              true,
				"/three/levels/before/simple-RE": true,
				"simple-RE-after":                true,
			},
		}, {
			RE:   "/simple-RE.*",
			Path: "simple-RE",
			Expected: map[string]bool{
				"/simple-RE":                    true,
				"/simple-RE/":                   true,
				"/simple-RE/after":              true,
				"/simple-RE/three/levels/after": true,
			},
		},
	}

	// test the "empty exclusion list" cases
	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tt.TestEqual(t, tw.shouldBeExcluded("/any/thing", false), false)
	tw.ExcludePath("")
	tt.TestEqual(t, tw.shouldBeExcluded("/any/thing", false), false)

	// test these cases on new instances of Tar object to avoid any
	// possible side effects/conflicts

	for _, tc := range testcases {
		w = bytes.NewBufferString("")
		tw = NewTar(w, makeTestDir(t))
		tw.ExcludePath(tc.RE)

		stdPaths := []string{
			tc.Path,
			"/" + tc.Path,
			tc.Path + "/",
			"/" + tc.Path + "/",
			"/before/" + tc.Path,
			"/" + tc.Path + "/after",
			"/before/" + tc.Path + "/after",
			"/three/levels/before/" + tc.Path,
			"/" + tc.Path + "/three/levels/after",
			"before-" + tc.Path,
			tc.Path + "-after",
			"before-" + tc.Path + "-after",
			tc.Path[:len(tc.Path)/2] + "-middle-" + tc.Path[len(tc.Path)/2:],
		}

		for _, path := range stdPaths {
			tt.TestEqual(t, tw.shouldBeExcluded(path, false), tc.Expected[path],
				fmt.Sprintf("Path:%q, tc:%v", path, tc))
			delete(tc.Expected, path)
		}

		for path, exp := range tc.Expected {
			tt.TestEqual(t, tw.shouldBeExcluded(path, false), exp)
		}
	}

	// This should return nil for these paths as they are excluded.
	// An extra check that processEntry indeed bails on excluded items
	w = bytes.NewBufferString("")
	tw = NewTar(w, makeTestDir(t))
	tw.ExcludePath("/one.*")
	tw.ExcludePath("/two/two/.*")
	tw.ExcludePath("/three/three/three.*")
	var fi staticFileInfo
	tt.TestExpectSuccess(t, tw.processEntry("/one/something", fi, []string{}))
	tt.TestExpectSuccess(t, tw.processEntry("/two/two/something", fi, []string{}))
	tt.TestExpectSuccess(t, tw.processEntry("/three/three/three-something", fi, []string{}))
}

func TestTarIDMapping(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	// set up our mapping funcs
	uidFuncCalled := false
	gidFuncCalled := false
	uidMappingFunc := func(uid int) (int, error) {
		uidFuncCalled = true
		return 0, nil
	}
	gidMappingFunc := func(gid int) (int, error) {
		gidFuncCalled = true
		return 0, nil
	}

	// set up our untar and use the test tar helper
	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.IncludeOwners = true
	tw.OwnerMappingFunc = uidMappingFunc
	tw.GroupMappingFunc = gidMappingFunc
	tt.TestExpectSuccess(t, tw.Archive())

	// untar it and verify all of the uid/gids are 0
	archive := tar.NewReader(w)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, header.Uid, 0)
		tt.TestEqual(t, header.Gid, 0)
	}
}

func TestSymlinkOptDereferenceLinkToFile(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	cwd, err := os.Getwd()
	tt.TestExpectSuccess(t, err)
	testHelper.AddTestFinalizer(func() {
		tt.TestExpectSuccess(t, os.Chdir(cwd))
	})

	dir := testHelper.TempDir()
	tt.TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	tt.TestExpectSuccess(t, os.Mkdir("a", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j", []byte{'t', 'e', 's', 't'}, mode))
	tt.TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	tt.TestExpectSuccess(t, os.Symlink("../i/j", "a/b/c/lj"))
	w := bytes.NewBufferString("")
	tw := NewTar(w, dir)
	tw.UserOptions |= c_DEREF
	tt.TestExpectSuccess(t, tw.Archive())

	extractionPath := path.Join(dir, "pkg")
	err = os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(w.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = dir
	tt.TestExpectSuccess(t, u.Extract())

	dirExists := func(name string) {
		f, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, true, f.IsDir())
	}

	sameFileContents := func(f1 string, f2 string) {
		b1, err := ioutil.ReadFile(f1)
		tt.TestExpectSuccess(t, err)

		b2, err := ioutil.ReadFile(f2)
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, b1, b2)
	}

	// Verify dirs a, a/b, a/b/c, a/b/c/d
	dirExists("./a")
	dirExists("./a/b")
	dirExists("./a/b/c")
	dirExists("./a/b/c/d")
	dirExists("./a/b/i")

	// Verify a/b/bash and /bin/bash are same
	sameFileContents(path.Join(extractionPath, "./a/b/bash"), "/bin/bash")

	// Verify that a/b/i/j and a/b/c/lj contents are same
	sameFileContents(path.Join(extractionPath, "./a/b/i/j"), path.Join(extractionPath, "./a/b/c/lj"))
}

func TestSymlinkOptDereferenceLinkToDir(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	cwd, err := os.Getwd()
	tt.TestExpectSuccess(t, err)
	testHelper.AddTestFinalizer(func() {
		tt.TestExpectSuccess(t, os.Chdir(cwd))
	})

	dir := testHelper.TempDir()
	tt.TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	tt.TestExpectSuccess(t, os.Mkdir("a", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j", []byte{'t', 'e', 's', 't'}, mode))
	tt.TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	tt.TestExpectSuccess(t, os.Symlink("../i", "a/b/c/l"))
	w := bytes.NewBufferString("")
	tw := NewTar(w, dir)
	tw.UserOptions |= c_DEREF
	tt.TestExpectSuccess(t, tw.Archive())

	extractionPath := path.Join(dir, "pkg")
	err = os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(w.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = dir
	tt.TestExpectSuccess(t, u.Extract())

	dirExists := func(name string) {
		f, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, true, f.IsDir())
	}

	sameFileContents := func(f1 string, f2 string) {
		b1, err := ioutil.ReadFile(f1)
		tt.TestExpectSuccess(t, err)

		b2, err := ioutil.ReadFile(f2)
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, b1, b2)
	}

	// Verify dirs a, a/b, a/b/c, a/b/c/d
	dirExists("./a")
	dirExists("./a/b")
	dirExists("./a/b/c")
	dirExists("./a/b/c/d")
	dirExists("./a/b/i")

	// Verify a/b/bash and /bin/bash are same
	sameFileContents(path.Join(extractionPath, "./a/b/bash"), "/bin/bash")

	// Verify that a/b/i/j and a/b/c/l/j contents are same
	sameFileContents(path.Join(extractionPath, "./a/b/i/j"), path.Join(extractionPath, "./a/b/c/l/j"))
}

func TestSymlinkOptDereferenceCircular(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	cwd, err := os.Getwd()
	tt.TestExpectSuccess(t, err)
	testHelper.AddTestFinalizer(func() {
		tt.TestExpectSuccess(t, os.Chdir(cwd))
	})

	dir := testHelper.TempDir()
	tt.TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	tt.TestExpectSuccess(t, os.Mkdir("a", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j", []byte{'t', 'e', 's', 't'}, mode))
	tt.TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	tt.TestExpectSuccess(t, os.Symlink(dir+"/a/b/c/l", "a/b/i/ll"))
	tt.TestExpectSuccess(t, os.Symlink("../i", "a/b/c/l"))
	w := bytes.NewBufferString("")
	tw := NewTar(w, dir)
	tw.UserOptions |= c_DEREF
	tt.TestExpectSuccess(t, tw.Archive())

	extractionPath := path.Join(dir, "pkg")
	err = os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(w.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = dir
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
	}

	dirExists := func(name string) {
		f, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, true, f.IsDir())
	}

	sameFileContents := func(f1 string, f2 string) {
		b1, err := ioutil.ReadFile(f1)
		tt.TestExpectSuccess(t, err)

		b2, err := ioutil.ReadFile(f2)
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, b1, b2)
	}

	// Verify dirs a, a/b, a/b/c, a/b/c/d
	dirExists("./a")
	dirExists("./a/b")
	dirExists("./a/b/c")
	dirExists("./a/b/c/d")
	dirExists("./a/b/i")

	// Verify that the file 'j' exists in both a/b/i and a/b/c/l
	fileExists("./a/b/i/j")
	fileExists("./a/b/c/l/j")

	// Verify a/b/bash
	sameFileContents(path.Join(extractionPath, "./a/b/bash"), "/bin/bash")

	// Verify that a/b/i/j and a/b/c/l/j contents are same
	sameFileContents(path.Join(extractionPath, "./a/b/i/j"), path.Join(extractionPath, "./a/b/c/l/j"))

	// Verify that the circular symbolic link a/b/i/ll does not exis
	_, err = os.Stat(path.Join(extractionPath, "./a/b/i/ll"))
	tt.TestEqual(t, true, os.IsNotExist(err))
}

func TestSymlinkOptDereferenceCircularToRoot(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	cwd, err := os.Getwd()
	tt.TestExpectSuccess(t, err)
	testHelper.AddTestFinalizer(func() {
		tt.TestExpectSuccess(t, os.Chdir(cwd))
	})

	dir := testHelper.TempDir()
	tt.TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	tt.TestExpectSuccess(t, os.Mkdir("a", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	tt.TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	tt.TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j", []byte{'t', 'e', 's', 't'}, mode))
	tt.TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	tt.TestExpectSuccess(t, os.Symlink(dir+"/a", "a/b/i/ll"))
	w := bytes.NewBufferString("")
	tw := NewTar(w, dir)
	tw.UserOptions |= c_DEREF
	tt.TestExpectSuccess(t, tw.Archive())

	extractionPath := path.Join(dir, "pkg")
	err = os.MkdirAll(extractionPath, 0755)
	tt.TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(w.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = dir
	tt.TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
	}

	dirExists := func(name string) {
		f, err := os.Stat(path.Join(extractionPath, name))
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, true, f.IsDir())
	}

	sameFileContents := func(f1 string, f2 string) {
		b1, err := ioutil.ReadFile(f1)
		tt.TestExpectSuccess(t, err)

		b2, err := ioutil.ReadFile(f2)
		tt.TestExpectSuccess(t, err)
		tt.TestEqual(t, b1, b2)
	}

	// Verify dirs a, a/b, a/b/c, a/b/c/d
	dirExists("./a")
	dirExists("./a/b")
	dirExists("./a/b/c")
	dirExists("./a/b/c/d")
	dirExists("./a/b/i")

	// Verify that the file 'j' exists in a/b/i
	fileExists("./a/b/i/j")

	// Verify a/b/bash
	sameFileContents(path.Join(extractionPath, "./a/b/bash"), "/bin/bash")

	// Verify that the circular symbolic link a/b/i/ll does not exist
	_, err = os.Stat(path.Join(extractionPath, "./a/b/i/ll"))
	tt.TestEqual(t, true, os.IsNotExist(err))
}

func TestTarPointedToFile(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	dir := testHelper.TempDir()
	apath := path.Join(dir, "a")

	// write the file, then read it the same way that we'll validate it
	tt.TestExpectSuccess(t, ioutil.WriteFile(apath, []byte("hello world"), os.FileMode(0644)))
	contents, err := ioutil.ReadFile(apath)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(contents), "hello world")

	// tar the file
	w := bytes.NewBufferString("")
	tw := NewTar(w, apath)
	tt.TestExpectSuccess(t, tw.Archive())

	// should then also be able to untar it
	dir = testHelper.TempDir()
	u := NewUntar(w, dir)
	u.AbsoluteRoot = dir
	tt.TestExpectSuccess(t, u.Extract())

	// stat it, ensure it exists and is a file, not a directory
	stat, err := os.Stat(path.Join(dir, "a"))
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, stat.IsDir(), false, "should be a file, not a directory")

	// read the contents to verify
	contents, err = ioutil.ReadFile(path.Join(dir, "a"))
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(contents), "hello world")
}

func TestTarPreserveSetuid(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	dir := testHelper.TempDir()
	apath := path.Join(dir, "a")

	err := ioutil.WriteFile(apath, []byte("hello world"), os.FileMode(0644))
	tt.TestExpectSuccess(t, err)

	err = os.Chmod(apath, os.FileMode(0644)|os.ModeSetuid)
	tt.TestExpectSuccess(t, err)

	w := bytes.NewBufferString("")
	tw := NewTar(w, apath)
	err = tw.Archive()
	tt.TestExpectSuccess(t, err)

	dir = testHelper.TempDir()
	u := NewUntar(w, dir)
	u.AbsoluteRoot = dir
	err = u.Extract()
	tt.TestExpectSuccess(t, err)

	stat, err := os.Stat(path.Join(dir, "a"))
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, stat.Mode()&os.ModeSetuid != 0, true, "must have setuid bit")
}

func TestTarCustomHandler(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.CustomHandlers = []TarCustomHandler{
		func(fullpath string, fi os.FileInfo, header *tar.Header) (bool, error) {
			if header.Name == "a/b/i/j/m" {
				header.Name = "a/b/i/j/n"
				header.Size = 0
				return true, nil
			}
			return false, nil
		},
	}
	tt.TestExpectSuccess(t, tw.Archive())

	archive := tar.NewReader(w)
	hasRenamedFile := false
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if header.Name == "a/b/i/j/m" {
			tt.Fatalf(t, "The \"a/b/i/j/m\" file should have been omitted")
		}
		if header.Name == "a/b/i/j/n" {
			hasRenamedFile = true
		}
	}

	tt.TestEqual(t, hasRenamedFile, true, "The tar file did not include the renamed file")
}

type staticFileInfo struct{}

func (m staticFileInfo) Name() string       { return "foo" }
func (m staticFileInfo) Size() int64        { return 1 }
func (m staticFileInfo) Mode() os.FileMode  { return 7777 }
func (m staticFileInfo) ModTime() time.Time { return time.Now() }
func (m staticFileInfo) IsDir() bool        { return false }
func (m staticFileInfo) Sys() interface{}   { return nil }
