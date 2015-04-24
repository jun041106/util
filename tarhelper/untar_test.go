// Copyright 2012-2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	. "github.com/apcera/util/testtool"
)

const CAPTURE_PRESERVE_TARBALL string = ""

func TestUntarResolveDestinations(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	u := new(Untar)
	u.resolvedLinks = make([]resolvedLink, 0)

	makeTestDir(t)

	runTest := func(p, e string) {
		dst, err := u.resolveDestination(p)
		TestExpectSuccess(t, err)
		TestEqual(t, e, dst)
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
	StartTest(t)
	defer FinishTest(t)

	// pre-clean /var/tmp from stale cruft
	for _, cruft := range []string{
		"/var/tmp/yy",
	} {
		if _, err := os.Stat(cruft); err == nil {
			err = os.Remove(cruft)
			if err != nil {
				t.Fatalf("cruft pre-clean error: %s", err)
			}
		}
	}

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
		TestExpectSuccess(t, archive.WriteHeader(header))
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

		TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		TestExpectSuccess(t, err)
		TestExpectSuccess(t, archive.Flush())
	}

	writeSymlink := func(name, link string) {
		header := new(tar.Header)
		header.Name = name
		header.Linkname = link
		header.Typeflag = tar.TypeSymlink
		header.Mode = 0644
		header.Mode |= c_ISLNK
		header.ModTime = time.Now()
		TestExpectSuccess(t, archive.WriteHeader(header))
	}

	// generate the mock tar
	writeDirectory(".")
	writeFile("./foo", "foo")
	writeDirectory("./usr")
	writeDirectory("./usr/bin")
	writeDirectory("./var")
	writeDirectory("./var/tmp")
	writeFile("./usr/bin/bash", "bash")
	writeSymlink("./usr/bin/sh", "bash")

	// now write a symlink that is an absolute path and then a file in it
	// absolute path will fail; marked as escaping when untar
	writeSymlink("./etc", "/realetc")
	//without the symlink the path for this still just gets made in a real /etc/zz dir
	writeFile("./etc/zz", "zz")
	// now also create a symlink that is an escape relative path and a file in it
	writeSymlink("./relativevartmp", "../../../../../../../../../../../var/tmp")
	writeFile("./relativevartmp/yy", "yy")
	writeSymlink("./relativeokay", "../var/tmp")
	writeFile("./relativeokay/xxgood", "xxgood")
	archive.Close()

	// debugging aid, be able to grab a copy of the generated tarball for
	// independent analysis; normally the const should be the empty string so
	// that this block doesn't happen (vulnerable to symlink attacks, etc)
	if CAPTURE_PRESERVE_TARBALL != "" {
		capR := bytes.NewReader(buffer.Bytes())
		capW, capErr := os.Create("/tmp/" + CAPTURE_PRESERVE_TARBALL + ".tar")
		if capErr != nil {
			t.Fatal(capErr.Error())
		}
		_, capErr = io.Copy(capW, capR)
		if capErr != nil {
			t.Fatal(capErr.Error())
		}
		capW.Close()
	}

	var err error

	// create temp folder to extract to
	tempDir := TempDir(t)
	extractionPath := path.Join(tempDir, "pkg")
	err = os.MkdirAll(extractionPath, 0755)
	TestExpectSuccess(t, err)
	err = os.MkdirAll(path.Join(tempDir, "realetc"), 0755)
	TestExpectSuccess(t, err)
	err = os.MkdirAll(path.Join(tempDir, "var", "tmp"), 0755)
	TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = tempDir
	TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		TestExpectSuccess(t, err, fmt.Sprintf("expected %q to exist", name))
	}

	absoluteFileNotExists := func(name string) {
		_, err := os.Stat(name)
		TestExpectError(t, err, fmt.Sprintf("did not expect %q to exist", name))
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
		TestEqual(t, string(b), contents)
	}

	fileSymlinks := func(name, link string) {
		l, err := os.Readlink(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
		TestEqual(t, l, link)
	}

	fileInvalidSymlink := func(name, link string) {
		_, err := os.Readlink(path.Join(tempDir, name))
		TestExpectError(t, err)
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
	fileInvalidSymlink("./pkg/etc", "/realetc")
	// Will fail currently
	fileExists("./realetc/zz")
	absoluteFileNotExists("/realetc/zz")
	fileContents("./realetc/zz", "zz")
	absoluteFileNotExists("/var/tmp/yy")
	fileExists("./var/tmp/xxgood")
	fileContents("./var/tmp/xxgood", "xxgood")
	fileExists("./pkg/relativeokay/xxgood")
	// If we decide to normalize links, instead of just skipping bad ones, then:
	//fileExists("./var/tmp/yy")
	//fileContents("./var/tmp/yy", "yy")
}

func TestUntarCreatesDeeperPathsIfNotMentioned(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

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

		TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		TestExpectSuccess(t, err)
		TestExpectSuccess(t, archive.Flush())
	}

	// generate the mock tar... this will write to a file in a directory that
	// isn't already created within the tar
	writeFile("./a_directory/file", "foo")
	archive.Close()

	// create temp folder to extract to
	tempDir := TempDir(t)
	extractionPath := path.Join(tempDir, "pkg")
	err := os.MkdirAll(extractionPath, 0755)
	TestExpectSuccess(t, err)

	// extract
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, extractionPath)
	u.AbsoluteRoot = tempDir
	TestExpectSuccess(t, u.Extract())

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
		TestEqual(t, string(b), contents)
	}

	fileExists("./pkg/a_directory/file")
	fileContents("./pkg/a_directory/file", "foo")
}

func TestUntarExtractOverwriting(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

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
		TestExpectSuccess(t, archive.WriteHeader(header))
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

		TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		TestExpectSuccess(t, err)
		TestExpectSuccess(t, archive.Flush())
	}

	writeSymlink := func(name, link string) {
		header := new(tar.Header)
		header.Name = name
		header.Linkname = link
		header.Typeflag = tar.TypeSymlink
		header.Mode = 0644
		header.Mode |= c_ISLNK
		header.ModTime = time.Now()
		TestExpectSuccess(t, archive.WriteHeader(header))
	}

	// create temp folder to extract to
	tempDir := TempDir(t)

	fileExists := func(name string) {
		_, err := os.Stat(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
	}

	fileContents := func(name, contents string) {
		b, err := ioutil.ReadFile(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
		TestEqual(t, string(b), contents)
	}

	fileSymlinks := func(name, link string) {
		l, err := os.Readlink(path.Join(tempDir, name))
		TestExpectSuccess(t, err)
		TestEqual(t, l, link)
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
	TestExpectSuccess(t, u.Extract())

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
	TestExpectSuccess(t, u.Extract())

	// verify the contents were overwritten as expected
	fileContents("./foo", "bar")
	fileContents("./usr/bin/zsh", "zsh")
	fileSymlinks("./usr/bin/sh", "zsh")
	fileContents("./etc", "etc")
	fileContents("./var/lib", "lll")
}

func TestUntarIDMappings(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

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
		TestExpectSuccess(t, archive.WriteHeader(header))
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

		TestExpectSuccess(t, archive.WriteHeader(header))
		_, err := archive.Write(b)
		TestExpectSuccess(t, err)
		TestExpectSuccess(t, archive.Flush())
	}

	writeDirectoryWithOwners(".", 0, 0)
	writeFileWithOwners("./foo", "foo", 0, 0)
	archive.Close()

	// setup our mapping func
	usr, err := user.Current()
	TestExpectSuccess(t, err)
	myUid, err := strconv.Atoi(usr.Uid)
	TestExpectSuccess(t, err)
	myGid, err := strconv.Atoi(usr.Gid)
	TestExpectSuccess(t, err)
	uidFuncCalled := false
	gidFuncCalled := false
	uidMappingFunc := func(uid int) (int, error) {
		uidFuncCalled = true
		TestEqual(t, uid, 0)
		return myUid, nil
	}
	gidMappingFunc := func(gid int) (int, error) {
		gidFuncCalled = true
		TestEqual(t, gid, 0)
		return myGid, nil
	}

	// extract
	tempDir := TempDir(t)
	r := bytes.NewReader(buffer.Bytes())
	u := NewUntar(r, tempDir)
	u.PreserveOwners = true
	u.OwnerMappingFunc = uidMappingFunc
	u.GroupMappingFunc = gidMappingFunc
	TestExpectSuccess(t, u.Extract())

	// verify it was called
	TestEqual(t, uidFuncCalled, true)
	TestEqual(t, gidFuncCalled, true)

	// verify the file
	stat, err := os.Stat(path.Join(tempDir, "foo"))
	TestExpectSuccess(t, err)
	sys := stat.Sys().(*syscall.Stat_t)
	TestEqual(t, sys.Uid, uint32(myUid))
	TestEqual(t, sys.Gid, uint32(myGid))
}

func TestUntarFailures(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	// Bad compression type.
	u := NewUntar(strings.NewReader("bad"), "/tmp")
	u.Compression = Compression(-1)
	TestExpectError(t, u.Extract())

	// FIXME(brady): add more cases here!
}

func TestCannotDetectCompression(t *testing.T) {
	StartTest(t)
	defer FinishTest(t)

	u := NewUntar(strings.NewReader("bad"), "/tmp")
	u.Compression = DETECT

	TestExpectError(t, u.Extract())
}
