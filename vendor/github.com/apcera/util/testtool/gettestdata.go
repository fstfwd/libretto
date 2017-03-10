package testtool

import (
	"path"
	"runtime"
	"strings"
	"testing"
)

type TestData struct {
	File       string
	Package    string
	TestName   string
	PackageDir string
	Line       int
}

// GetTestData goes through the call stack of the current goroutine and creates
// a ordered set of strings for the files, function names, and line numbers then
// adds this data to the error message.
func GetTestData(t *testing.T) *TestData {
	var pcs [20]uintptr
	pcCount := runtime.Callers(2, pcs[:])
	pcCount -= 2

	for _, pc := range pcs[0:pcCount] {
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc - 1)
		basePkgDir, pkgFname := path.Split(f.Name())
		fname := path.Ext(pkgFname)
		pkg := strings.TrimSuffix(pkgFname, fname)
		fname = strings.TrimLeft(fname, ".")
		pkgDir := path.Join(basePkgDir, pkg)

		if strings.HasPrefix(fname, "Test") {
			return &TestData{
				File:       file,
				Line:       line,
				TestName:   fname,
				Package:    pkg,
				PackageDir: pkgDir,
			}
		}
	}

	return nil
}
