// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/apcera/logging/unittest"
)

// -----------------------------------------------------------------------
// Initialization, cleanup, and shutdown functions.
// -----------------------------------------------------------------------

// Stores output from the logging system so it can be written only if
// the test actually fails.
var logBuffer unittest.LogBuffer = unittest.SetupBuffer()

// This is a list of functions that will be run on test completion. Having
// this allows us to clean up temporary directories or files after the
// test is done which is a huge win.
var Finalizers []func() = nil

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
	logBuffer.FinishTest(t)
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
	f, err := ioutil.TempDir("", "golangunittest")
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
