// Copyright 2012-2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// Tar manages state for a TAR archive.
type Tar struct {
	target string

	// The destination writer
	dest io.Writer

	// The archive/tar reader that we will use to extract each
	// element from the tar file. This will be set when Extract()
	// is called.
	archive *tar.Writer

	// The Compression being used in this tar.
	Compression Compression

	// Set to true if archiving should attempt to preserve
	// permissions as it was on the filesystem. If this is false then
	// files will be archived with basic file/directory permissions.
	IncludePermissions bool

	// Set to true to perserve ownership of files and directories. If set to
	// false, the Uid and Gid will be set as 500, which is the first Uid/Gid
	// reserved for normal users.
	IncludeOwners bool

	// If set, this will be a virtual path that is prepended to the
	// file location.  This allows the target to be under a temp directory
	// but have it packaged as though it was under another directory, such as
	// taring /tmp/build, and having
	//   /tmp/build/bin/foo be /var/lib/build/bin/foo
	// in the tar archive.
	VirtualPath string

	// This is used to track potential hard links. We check the number of links
	// and push the inode on here when archiving to see if we run across the
	// inode again later.
	hardLinks map[uint64]string
}

// Mode constants from the tar spec.
const (
	c_ISUID  = 04000 // Set uid
	c_ISGID  = 02000 // Set gid
	c_ISDIR  = 040000
	c_ISFIFO = 010000
	c_ISREG  = 0100000
	c_ISLNK  = 0120000
	c_ISBLK  = 060000
	c_ISCHR  = 020000
	c_ISSOCK = 0140000
)

// NewTar returns a Tar ready to write the contents of targetDir to w.
func NewTar(w io.Writer, targetDir string) *Tar {
	return &Tar{
		target:             targetDir,
		dest:               w,
		hardLinks:          make(map[uint64]string),
		IncludePermissions: true,
		IncludeOwners:      false,
	}
}

func (t *Tar) Archive() error {
	defer func() {
		if t.archive != nil {
			t.archive.Close()
			t.archive = nil
		}
	}()

	// Create a TarWriter that wraps the proper io.Writer object
	// the implements the expected compression for this file.
	switch t.Compression {
	case NONE:
		t.archive = tar.NewWriter(t.dest)
	case GZIP:
		dest := gzip.NewWriter(t.dest)
		defer dest.Close()
		t.archive = tar.NewWriter(dest)
	case BZIP2:
		return fmt.Errorf("bzip2 compression is not supported")
	case DETECT:
		return fmt.Errorf("not a valid compression type: %s", DETECT)
	default:
		return fmt.Errorf("unknown compression type: %s", t.Compression)
	}

	// ensure we write the current directory
	f, err := os.Stat(t.target)
	if err != nil {
		return err
	}

	// walk the directory tree
	if err := t.processEntry(".", f); err != nil {
		return err
	}

	return nil
}

func (t *Tar) processDirectory(dir string) error {
	// get directory entries
	files, err := ioutil.ReadDir(path.Join(t.target, dir))
	if err != nil {
		return err
	}

	for _, f := range files {
		fullName := path.Join(dir, f.Name())
		if err := t.processEntry(fullName, f); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tar) processEntry(fullName string, f os.FileInfo) error {
	var err error

	// set base header parameters
	header := new(tar.Header)
	header.Name = "./" + fullName
	header.Mode = int64(f.Mode().Perm())
	header.ModTime = f.ModTime()

	// handle VirtualPath
	if t.VirtualPath != "" {
		header.Name = path.Clean(path.Join(".", t.VirtualPath, header.Name))
	}

	// copy uid/gid if Permissions enabled
	stat := f.Sys().(*syscall.Stat_t)
	if t.IncludeOwners {
		header.Uid = int(stat.Uid)
		header.Gid = int(stat.Gid)
	} else {
		header.Uid = 500
		header.Gid = 500
	}

	mode := f.Mode()
	switch {
	// directory handling
	case f.IsDir():
		// if Permissions is not enabled, force mode back to 0755
		if !t.IncludePermissions {
			header.Mode = 0755
		}

		// update directory specific values, tarballs often append with a slash
		header.Name = header.Name + "/"
		header.Typeflag = tar.TypeDir
		header.Mode |= c_ISDIR

		// write the header
		err = t.archive.WriteHeader(header)
		if err != nil {
			return err
		}
		// process the directory's entries next
		if err = t.processDirectory(fullName); err != nil {
			return err
		}

	// symlink handling
	case mode&os.ModeSymlink == os.ModeSymlink:
		// if Permissions is not enabled, force mode back to 0755
		if !t.IncludePermissions {
			header.Mode = 0755
		}

		// symlink type flags
		header.Typeflag = tar.TypeSymlink
		header.Mode |= c_ISLNK

		// read and process the link
		link, err := cleanLinkName(t.target, fullName)
		if err != nil {
			return err
		}
		header.Linkname = link

		// write the header
		err = t.archive.WriteHeader(header)
		if err != nil {
			return err
		}

	// regular file handling
	case mode&os.ModeType == 0:
		// if Permissions is not enabled, force mode back to 0644
		if !t.IncludePermissions {
			header.Mode = 0644
		}

		// base file flags
		header.Size = f.Size()
		header.Typeflag = tar.TypeReg
		header.Mode |= c_ISREG

		// check to see if this is a hard link
		if stat.Nlink > 1 {
			if dst, ok := t.hardLinks[stat.Ino]; ok {
				// update the header if it is
				header.Typeflag = tar.TypeLink
				header.Linkname = dst
				header.Size = 0
			} else {
				// push it on the list, and continue to write it as a file
				// this is our first time seeing it
				t.hardLinks[stat.Ino] = header.Name
			}
		}

		// write the header
		err = t.archive.WriteHeader(header)
		if err != nil {
			return err
		}

		// only write the file if tye type is still a regular file
		if header.Typeflag == tar.TypeReg {
			// open the file and copy
			data, err := os.Open(path.Join(t.target, fullName))
			if err != nil {
				return err
			}
			_, err = io.Copy(t.archive, data)
			if err != nil {
				data.Close()
				return err
			}

			// important to flush before the file is closed
			err = t.archive.Flush()
			if err != nil {
				data.Close()
				return err
			}
			// we want to ensure the file is closed in the loop
			data.Close()
		}

	// device support
	case mode&os.ModeDevice == os.ModeDevice ||
		mode&os.ModeCharDevice == os.ModeCharDevice:
		//
		// get the mode to OR by
		devmode := int64(0)
		switch {
		case mode&os.ModeDevice == os.ModeDevice:
			devmode = c_ISBLK
		case mode&os.ModeCharDevice == os.ModeCharDevice:
			devmode = c_ISCHR
		}

		// set other flags
		header.Typeflag = tar.TypeBlock
		header.Mode |= devmode

		// stat to get devmode
		fi, err := os.Stat(path.Join(t.target, fullName))
		if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
			header.Devmajor = majordev(int64(sys.Rdev))
			header.Devminor = minordev(int64(sys.Rdev))
		}

		// write the header
		err = t.archive.WriteHeader(header)
		if err != nil {
			return err
		}

	// socket handling
	case mode&os.ModeSocket == os.ModeSocket:
		// skip... gnutar does, so we will
	default:
	}

	return nil
}

func cleanLinkName(targetDir, name string) (string, error) {
	dir := filepath.Dir(name)

	// read the link
	link, err := os.Readlink(path.Join(targetDir, name))
	if err != nil {
		return "", err
	}

	// if the target isn't absolute, make it absolute
	// even if it is absolute, we want to convert it to be relative
	if !path.IsAbs(link) {
		link, err = filepath.Abs(path.Join(targetDir, dir, link))
		if err != nil {
			return "", err
		}
	}

	// do a quick clean pass
	link = path.Clean(link)

	// if the link path contains the target path, then convert the link to be
	// relative. this ensures it is properly preserved whereever it is later
	// extracted. if it is a path outside the target, then preserve it as an
	// absolute path
	if strings.Contains(link, targetDir) {
		// remove the targetdir to ensure the link is relative
		link, err = filepath.Rel(path.Join(targetDir, dir), link)
		if err != nil {
			return "", err
		}
	}

	return link, nil
}
