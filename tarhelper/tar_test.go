// Copyright 2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bytes"
	"io"
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

func TestPathExclusion(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	w := bytes.NewBufferString("")
	tw := NewTar(w, makeTestDir(t))
	fooPath := "/foo/bar"
	tmpPath := "/tmp"
	pidPattern := "*.pid"
	tw.ExcludePath(fooPath)
	tw.ExcludePath(tmpPath)
	tw.ExcludePath(pidPattern)
	TestEqual(t, len(tw.ExcludedPaths), 3)

	TestEqual(t, tw.shouldBeExcluded(fooPath), false)
	TestEqual(t, tw.shouldBeExcluded(tmpPath), true)
	TestEqual(t, tw.shouldBeExcluded(fooPath[1:]), true)
	TestEqual(t, tw.shouldBeExcluded(tmpPath[1:]), true)
	TestEqual(t, tw.shouldBeExcluded("/baz/bar"), false)
	TestEqual(t, tw.shouldBeExcluded("foobar.pid"), true)
	TestEqual(t, tw.shouldBeExcluded("/foo/bar/path/pid.pid"), true)

	// This should return nil for these paths as they are excluded.
	TestEqual(t, tw.processEntry(fooPath[1:], nil), nil)
	TestEqual(t, tw.processEntry(tmpPath[1:], nil), nil)
}

func TestTarIDMapping(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

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
	TestExpectSuccess(t, tw.Archive())

	// untar it and verify all of the uid/gids are 0
	archive := tar.NewReader(w)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		TestExpectSuccess(t, err)
		TestEqual(t, header.Uid, 0)
		TestEqual(t, header.Gid, 0)
	}
}
