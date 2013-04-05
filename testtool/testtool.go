// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/apcera/logging"
	"github.com/apcera/logging/unittest"
)

// Common interface that can be used to allow testing.B and testing.T objects
// to by passed to the same function.
type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// -----------------------------------------------------------------------
// Initialization, cleanup, and shutdown functions.
// -----------------------------------------------------------------------

// Stores output from the logging system so it can be written only if
// the test actually fails.
var LogBuffer unittest.LogBuffer = unittest.SetupBuffer()

// This is a list of functions that will be run on test completion. Having
// this allows us to clean up temporary directories or files after the
// test is done which is a huge win.
var Finalizers []func() = nil

// Adds a function to be called once the test finishes.
func AddTestFinalizer(f func()) {
	Finalizers = append(Finalizers, f)
}

// Called at the start of a test to setup all the various state bits that
// are needed. All tests in this module should start by calling this
// function.
func StartTest(t *testing.T) {
}

// Called as a defer to a test in order to clean up after a test run. All
// tests in this module should call this function as a defer right after
// calling StartTest()
func FinishTest(t *testing.T) {
	for i := range Finalizers {
		Finalizers[len(Finalizers)-1-i]()
	}
	Finalizers = nil
	LogBuffer.FinishTest(t)
}

// -----------------------------------------------------------------------
// Temporary file helpers.
// -----------------------------------------------------------------------

// Writes contents to a temporary file, sets up a Finalizer to remove
// the file once the test is complete, and then returns the newly
// created filename to the caller.
func WriteTempFile(t *testing.T, contents string) string {
	f, err := ioutil.TempFile("", "golangunittest")
	if f == nil {
		t.Fatalf("ioutil.TempFile() return nil.")
	} else if err != nil {
		t.Fatalf("ioutil.TempFile() return an err: %s", err)
	}
	defer f.Close()
	Finalizers = append(Finalizers, func() {
		os.Remove(f.Name())
	})
	contentsBytes := []byte(contents)
	n, err := f.Write(contentsBytes)
	if err != nil {
		t.Fatalf("Error writing to %s: %s", f.Name(), err)
	} else if n != len(contentsBytes) {
		t.Fatalf("Short write to %s", f.Name())
	}
	return f.Name()
}

// Makes a temporary directory
func TempDir(t *testing.T) string {
	f, err := ioutil.TempDir(RootTempDir(t), "golangunittest")
	if f == "" {
		t.Fatalf("ioutil.TempFile() return an empty string.")
	} else if err != nil {
		t.Fatalf("ioutil.TempFile() return an err: %s", err)
	}

	Finalizers = append(Finalizers, func() {
		os.RemoveAll(f)
	})
	return f
}

// Allocate a temporary file and ensure that it gets cleaned up when the
// test is completed.
func TempFile(t *testing.T) string {
	file, err := ioutil.TempFile(RootTempDir(t), "unittest")
	if err != nil {
		Fatalf(t, "Error making temporary file: %s", err)
	}
	defer file.Close()
	name := file.Name()
	Finalizers = append(Finalizers, func() {
		os.RemoveAll(name)
	})
	return name
}

// -----------------------------------------------------------------------
// Fatalf wrapper.
// -----------------------------------------------------------------------

// This function wraps Fatalf in order to provide a functional stack trace
// on failures rather than just a line number of the failing check. This
// helps if you have a test that fails in a loop since it will show
// the path to get there as well as the error directly.
func Fatalf(t Logger, f string, args ...interface{}) {
	lines := make([]string, 0, 100)
	msg := fmt.Sprintf(f, args...)
	lines = append(lines, msg)

	// Generate the Stack of callers:
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok == false {
			break
		}
		msg := fmt.Sprintf("%d - %s:%d", i, file, line)
		lines = append(lines, msg)
	}

	logging.Errorf("Test has failed: %s", msg)
	t.Fatalf("%s", strings.Join(lines, "\n"))
}

// -----------------------------------------------------------------------
// Simple Timeout functions
// -----------------------------------------------------------------------

// runs the given function until 'timeout' has passed, sleeping 'sleep'
// duration in between runs. If the function returns true this exits,
// otherwise after timeout this will fail the test.
func Timeout(
	t *testing.T, timeout time.Duration, sleep time.Duration,
	f func() bool) {
	//
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if f() == true {
			return
		}
		time.Sleep(sleep)
	}
	Fatalf(t, "Timeout.")
}
