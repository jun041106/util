// Copyright 2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/apcera/util/testtool"
)

func makeTestDir(t *testing.T) string {
	cwd, err := os.Getwd()
	TestExpectSuccess(t, err)
	AddTestFinalizer(func() {
		TestExpectSuccess(t, os.Chdir(cwd))
	})
	dir := TempDir(t)
	TestExpectSuccess(t, os.Chdir(dir))
	mode := os.FileMode(0755)
	TestExpectSuccess(t, os.Mkdir("a", mode))
	TestExpectSuccess(t, os.Mkdir("a/b", mode))
	TestExpectSuccess(t, os.Mkdir("a/b/c", mode))
	TestExpectSuccess(t, os.Mkdir("a/b/c/d", mode))
	TestExpectSuccess(t, os.Mkdir("a/b/i", mode))
	TestExpectSuccess(t, os.Mkdir("a/b/i/j", mode))
	TestExpectSuccess(t, ioutil.WriteFile("a/b/c/d/e", []byte{}, mode))
	TestExpectSuccess(t, ioutil.WriteFile("a/b/c/f", []byte{}, mode))
	TestExpectSuccess(t, ioutil.WriteFile("a/b/g", []byte{}, mode))
	TestExpectSuccess(t, ioutil.WriteFile("a/b/i/j/k", []byte{}, mode))
	TestExpectSuccess(t, os.Symlink("/bin/bash", "a/b/bash"))
	TestExpectSuccess(t, os.Symlink("../i", "a/b/c/l"))
	TestExpectSuccess(t, os.Symlink("g", "a/b/h"))
	TestExpectSuccess(t, os.Symlink("k", "a/b/i/j/l"))
	TestExpectSuccess(t, os.Symlink("../../g", "a/b/i/j/m"))
	return dir
}

func TestTarSimple(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	TestExpectSuccess(t, tw.Archive())
}

func TestTarVirtualPath(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	tw.VirtualPath = "foo"
	TestExpectSuccess(t, tw.Archive())
}
