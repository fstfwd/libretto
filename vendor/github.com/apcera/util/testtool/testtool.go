// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/apcera/logray"
	"github.com/apcera/logray/unittest"
)

// Logger is a common interface that can be used to allow testing.B and
// testing.T objects to be passed to the same function.
type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

// Backtracer is an interface that provies additional information to be
// displayed using the TestExpectSuccess() functions. For an example see
// BackError in the apcera/cfg package.
type Backtracer interface {
	Backtrace() []string
}

// -----------------------------------------------------------------------
// Initialization, cleanup, and shutdown functions.
// -----------------------------------------------------------------------

// If this flag is set to true then output will be displayed live as it
// happens rather than being buffered and only displayed when tests fail.
var streamTestOutput bool

// If a -log or log is provided with an path to a directory then that path is
// available in this variable. This is a helper for tests that wish to log. An
// empty string indicates the path was not set. The value is set only to allow
// callers to make use of in their tests. There are no other side effects.
var TestLogFile string = ""

func init() {
	if f := flag.Lookup("log"); f == nil {
		flag.StringVar(
			&TestLogFile,
			"log",
			"",
			"Specifies the log file for the test")
	}
	if f := flag.Lookup("live-output"); f == nil {
		flag.BoolVar(
			&streamTestOutput,
			"live-output",
			false,
			"Enable output to be streamed live rather than buffering.")
	}
}

// TestTool type allows for parallel tests.
type TestTool struct {
	*testing.T

	// Stores output from the logging system so it can be written only if
	// the test actually fails.
	LogBuffer *unittest.LogBuffer

	// This is a list of functions that will be run on test completion. Having
	// this allows us to clean up temporary directories or files after the
	// test is done which is a huge win.
	Finalizers []func()

	// Parameters contains test-specific caches of data.
	Parameters map[string]interface{}

	RandomTestString string
	PackageHash      string

	*TestData
}

// AddTestFinalizer adds a function to be called once the test finishes.
func (tt *TestTool) AddTestFinalizer(f func()) {
	tt.Finalizers = append(tt.Finalizers, f)
}

// StartTest should be called at the start of a test to setup all the various
// state bits that are needed.
func StartTest(t *testing.T) *TestTool {
	tt := TestTool{
		Parameters:       make(map[string]interface{}),
		T:                t,
		RandomTestString: RandomTestString(10),
	}

	tt.TestData = GetTestData(t)

	if tt.TestData == nil {
		panic("Failed to read information about the test.")
	}

	tt.PackageHash = tt.Package + hashPackage(tt.PackageDir)

	if !streamTestOutput {
		tt.LogBuffer = unittest.SetupBuffer()
	} else {
		logray.AddDefaultOutput("stdout://", logray.ALL)
	}

	return &tt
}

// FinishTest is called as a defer to a test in order to clean up after a test
// run. All tests in this module should call this function as a defer right
// after calling StartTest()
func (tt *TestTool) FinishTest() {
	for i := len(tt.Finalizers) - 1; i >= 0; i-- {
		tt.Finalizers[i]()
	}
	tt.Finalizers = nil
	if tt.LogBuffer != nil {
		tt.LogBuffer.FinishTest(tt.T)
	}
}

// TestRequiresRoot is called to require that your test is run as root. NOTICE:
// this does not cause the test to FAIL. This seems like the most sane thing to
// do based on the shortcomings of Go's test utilities.
//
// As an added feature this function will append all skipped test names into
// the file name specified in the environment variable:
//   $SKIPPED_ROOT_TESTS_FILE
func TestRequiresRoot(l Logger) {
	getTestName := func() string {
		// Maximum function depth. This shouldn't be called when the stack is
		// 1024 calls deep (its typically called at the top of the Test).
		pc := make([]uintptr, 1024)
		callers := runtime.Callers(2, pc)
		testname := ""
		for i := 0; i < callers; i++ {
			if f := runtime.FuncForPC(pc[i]); f != nil {
				// Function names have the following formats:
				//   runtime.goexit
				//   testing.tRunner
				//   github.com/util/testtool.TestRequiresRoot
				// To find the real function name we split on . and take the
				// last element.
				names := strings.Split(f.Name(), ".")
				if strings.HasPrefix(names[len(names)-1], "Test") {
					testname = names[len(names)-1]
				}
			}
		}
		if testname == "" {
			Fatalf(l, "Can't figure out the test name.")
		}
		return testname
	}

	if os.Getuid() != 0 {
		// We support the ability to set an environment variables where the
		// names of all skipped tests will be logged. This is used to ensure
		// that they can be run with sudo later.
		fn := os.Getenv("SKIPPED_ROOT_TESTS_FILE")
		if fn != "" {
			// Get the test name. We do this using the runtime package. The
			// first function named Test* we assume is the outer test function
			// which is in turn the test name.
			flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
			f, err := os.OpenFile(fn, flags, os.FileMode(0644))
			TestExpectSuccess(l, err)
			defer f.Close()
			_, err = f.WriteString(getTestName() + "\n")
			TestExpectSuccess(l, err)
		}

		l.Skipf("This test must be run as root. Skipping.")
	}
}

// -----------------------------------------------------------------------
// Temporary file helpers.
// -----------------------------------------------------------------------

// WriteTempFile writes contents to a temporary file, sets up a Finalizer to
// remove the file once the test is complete, and then returns the newly created
// filename to the caller.
func (tt *TestTool) WriteTempFile(contents string) string {
	return tt.WriteTempFileMode(contents, os.FileMode(0644))
}

// WriteTempFileMode is like WriteTempFile but sets the mode.
func (tt *TestTool) WriteTempFileMode(contents string, mode os.FileMode) string {
	f, err := ioutil.TempFile("", "golangunittest")
	if f == nil {
		Fatalf(tt.T, "ioutil.TempFile() return nil.")
	} else if err != nil {
		Fatalf(tt.T, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(tt.T, "os.Chmod() returned an error: %s", err)
	}
	defer f.Close()
	tt.Finalizers = append(tt.Finalizers, func() {
		os.Remove(f.Name())
	})
	contentsBytes := []byte(contents)
	n, err := f.Write(contentsBytes)
	if err != nil {
		Fatalf(tt.T, "Error writing to %s: %s", f.Name(), err)
	} else if n != len(contentsBytes) {
		Fatalf(tt.T, "Short write to %s", f.Name())
	}
	return f.Name()
}

// TempDir makes a temporary directory.
func (tt *TestTool) TempDir() string {
	return tt.TempDirMode(os.FileMode(0755))
}

// TempDirMode makes a temporary directory with the given mode.
func (tt *TestTool) TempDirMode(mode os.FileMode) string {
	f, err := ioutil.TempDir(RootTempDir(tt), "golangunittest")
	if f == "" {
		Fatalf(tt.T, "ioutil.TempFile() return an empty string.")
	} else if err != nil {
		Fatalf(tt.T, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f, mode); err != nil {
		Fatalf(tt.T, "os.Chmod failure.")
	}

	tt.Finalizers = append(tt.Finalizers, func() {
		os.RemoveAll(f)
	})
	return f
}

// TempFile allocate a temporary file and ensures that it gets cleaned up when
// the test is completed.
func (tt *TestTool) TempFile() string {
	return tt.TempFileMode(os.FileMode(0644))
}

// TempFileMode writes a temp file with the given mode.
func (tt *TestTool) TempFileMode(mode os.FileMode) string {
	f, err := ioutil.TempFile(RootTempDir(tt), "unittest")
	if err != nil {
		Fatalf(tt.T, "Error making temporary file: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(tt.T, "os.Chmod failure.")
	}
	defer f.Close()
	name := f.Name()
	tt.Finalizers = append(tt.Finalizers, func() {
		os.RemoveAll(name)
	})
	return name
}

// -----------------------------------------------------------------------
// Fatalf wrapper.
// -----------------------------------------------------------------------

// Fatalf wraps Fatalf in order to provide a functional stack trace on failures
// rather than just a line number of the failing check. This helps if you have a
// test that fails in a loop since it will show the path to get there as well as
// the error directly.
func Fatalf(l Logger, f string, args ...interface{}) {
	lines := make([]string, 0, 100)
	msg := fmt.Sprintf(f, args...)
	lines = append(lines, msg)

	// Get the directory of testtool in order to ensure that we don't show
	// it in the stack traces (it can be spammy).
	_, myfile, _, _ := runtime.Caller(0)
	mydir := path.Dir(myfile)

	// Generate the Stack of callers:
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok == false {
			break
		}
		// Don't print the stack line if its within testtool since its
		// annoying to see the testtool internals.
		if path.Dir(file) == mydir {
			continue
		}
		msg := fmt.Sprintf("%d - %s:%d", i, file, line)
		lines = append(lines, msg)
	}
	l.Fatalf("%s", strings.Join(lines, "\n"))
}

// Fatal fails the test with a simple format for the message.
func Fatal(t Logger, args ...interface{}) {
	Fatalf(t, "%s", fmt.Sprint(args...))
}

// -----------------------------------------------------------------------
// Simple Timeout functions
// -----------------------------------------------------------------------

// Timeout runs the given function until 'timeout' has passed, sleeping 'sleep'
// duration in between runs. If the function returns true this exits, otherwise
// after timeout this will fail the test.
func Timeout(l Logger, timeout, sleep time.Duration, f func() bool) {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if f() == true {
			return
		}
		time.Sleep(sleep)
	}
	Fatalf(l, "testtool: Timeout after %v", timeout)
}

// -----------------------------------------------------------------------
// Error object handling functions.
// -----------------------------------------------------------------------

// TestExpectError calls Fatal if err is nil.
func TestExpectError(l Logger, err error, msg ...string) {
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if err == nil {
		Fatalf(l, "Expected an error, got nil%s", reason)
	}
}

// isRealError detects if a nil of a type stored as a concrete type, rather than
// an error interface, was passed in.
func isRealError(err error) bool {
	if err == nil {
		return false
	}
	v := reflect.ValueOf(err)
	if !v.CanInterface() {
		return true
	}
	if v.IsNil() {
		return false
	}
	return true
}

// TestExpectSuccess fails the test if err is not nil and fails the test and
// output the reason for the failure as the err argument the same as Fatalf. If
// err implements the BackTracer interface a backtrace will also be displayed.
func TestExpectSuccess(l Logger, err error, msg ...string) {
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if err != nil && isRealError(err) {
		lines := make([]string, 0, 50)
		lines = append(lines, fmt.Sprintf("Unexpected error: %s", err))
		if be, ok := err.(Backtracer); ok {
			for _, line := range be.Backtrace() {
				lines = append(lines, fmt.Sprintf(" * %s", line))
			}
		}
		Fatalf(l, "%s%s", strings.Join(lines, "\n"), reason)
	}
}

// TestExpectNonZeroLength fails the test if the given value is not zero.
func TestExpectZeroLength(l Logger, size int) {
	if size != 0 {
		Fatalf(l, "Zero length expected")
	}
}

// TestExpectNonZeroLength fails the test if the given value is zero.
func TestExpectNonZeroLength(l Logger, size int) {
	if size == 0 {
		Fatalf(l, "Zero length found")
	}
}

// TestExpectPanic verifies that a panic is called with the expected msg.
func TestExpectPanic(l Logger, f func(), msg string) {
	defer func(msg string) {
		if m := recover(); m != nil {
			TestEqual(l, msg, m)
		}
	}(msg)
	f()
	Fatalf(l, "Expected a panic with message '%s'\n", msg)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomTestString generates a random test string from only upper and lower
// case letters.
func RandomTestString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

var encoder = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789__")

func hashPackage(pkg string) string {
	hash := md5.New()
	hash.Write([]byte(pkg))
	out := encoder.EncodeToString(hash.Sum(nil))
	//Ensure alphanumber prefix and remove base64 padding
	return "p" + out[:len(out)-2]
}
