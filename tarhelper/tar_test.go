// Copyright 2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bytes"
	"fmt"
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
	TestEqual(t, tw.shouldBeExcluded("/any/thing"), false)
	tw.ExcludePath("")
	TestEqual(t, tw.shouldBeExcluded("/any/thing"), false)

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
			TestEqual(t, tw.shouldBeExcluded(path), tc.Expected[path],
				fmt.Sprintf("Path:%q, tc:%v", path, tc))
			delete(tc.Expected, path)
		}

		for path, exp := range tc.Expected {
			TestEqual(t, tw.shouldBeExcluded(path), exp)
		}
	}

	// This should return nil for these paths as they are excluded.
	// An extra check that processEntry indeed bails on excluded items
	w = bytes.NewBufferString("")
	tw = NewTar(w, makeTestDir(t))
	tw.ExcludePath("/one.*")
	tw.ExcludePath("/two/two/.*")
	tw.ExcludePath("/three/three/three.*")
	TestExpectSuccess(t, tw.processEntry("/one/something", nil))
	TestExpectSuccess(t, tw.processEntry("/two/two/something", nil))
	TestExpectSuccess(t, tw.processEntry("/three/three/three-something", nil))
	TestExpectError(t, tw.processEntry("/two/two-something", nil))
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
