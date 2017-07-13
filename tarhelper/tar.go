// Copyright 2012-2016 Apcera Inc. All rights reserved.

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
	"regexp"
	"strings"
)

// User options enumeration type. This encodes the control options provided
// by user.
type UserOption int

// DirStack tracks circular symbolic links for the dereference archive option.
// Declaring a type here to highlight the semantics.
type DirStack []string

// ignoreInfo expands the idea of excluding a path by also specifying metadata
// about the regexp and how to process a match.
type ignoreInfo struct {
	// regexp is the regular expression responsible for deciding matches.
	regexp *regexp.Regexp

	// exclude specifies whether or not the matched file should be excluded or
	// included. This allows subsequent matches to reinclude files previously
	// excluded.
	exclude bool

	// dirOnly will consider the regexp a match only if it is also a directory.
	dirOnly bool
}

// TarCustomHandler are used to inject custom behavior for handling file entries
// going into a tar file. For more information, see Tar.CustomerHandlers
// description.
type TarCustomHandler func(fullpath string, fi os.FileInfo, header *tar.Header) (bool, error)

// TarCustomHook can inject additional header and file data into the archive. For
// more information, see Tar.PrefixHook and Tar.SuffixHook.
type TarCustomHook func(archive *tar.Writer) error

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

	// ignorePaths contains any paths that a user may want to exclude from the
	// tar. Anything included in any paths set on this field will not be
	// included in the tar.
	ignorePaths []ignoreInfo

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

	// OwnerMappingFunc is used to give the caller the ability to control the
	// mapping of UIDs in the tar into what they should be on the host. The
	// function is only used when IncludeOwners is true. The function is passed in
	// the UID of the file on the filesystem and is expected to return a UID to
	// use within the tar file. It can also return an error if it is unable to
	// choose a UID or the UID is not allowed.
	OwnerMappingFunc func(int) (int, error)

	// GroupMappingFunc is used to give the caller the ability to control the
	// mapping of GIDs in the tar into what they should be on the host. The
	// function is only used when IncludeOwners is true. The function is passed in
	// the GID of the file on the filesystem and is expected to return a GID to
	// use within the tar file. It can also return an error if it is unable to
	// choose a GID or the GID is not allowed.
	GroupMappingFunc func(int) (int, error)

	// ExcludeRootPath ensures the resulting tarball does not include
	// a header entry for "./". This prevents untarring from modifying
	// the parent directory.
	ExcludeRootPath bool

	// User provided control options. UserOption enum has the
	// definitions and explanations for the various flags.
	UserOptions UserOption

	// CustomHandlers is used to allow the code calling tarhelper to inject custom
	// logic for how to handle certain entries being written to the tar file. The
	// Tar handler will loop over and call to these functions. They return a
	// boolean which should be true when the built in logic for handling the file
	// should be skipped. They also return an error which will cause the tar
	// function to abort and bubble up the handler's error. The functions are
	// passed the path where the entry are located on disk, the os.FileInfo for
	// the file, and the *tar.Header entry for it.
	CustomHandlers []TarCustomHandler

	// PrefixHook executes before the file system is traversed and can be used to inject
	// content into the archive which does not exist within the file system tree. This
	// content will be extracted before any file system data.
	PrefixHook TarCustomHook

	// SuffixHook executes after the file system is traversed and like PrefixHook can be
	// used to inject additional content into the archive. This content will be extracted
	// after data from the file system.
	SuffixHook TarCustomHook
}

// UserOption definitions.
const (
	c_DEREF UserOption = 1 << iota // Follow symbolic links when archiving.
)

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
		OwnerMappingFunc:   defaultMappingFunc,
		GroupMappingFunc:   defaultMappingFunc,
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
		return fmt.Errorf("not a valid compression type: %v", DETECT)
	default:
		return fmt.Errorf("unknown compression type: %v", t.Compression)
	}

	// ensure the target exists
	f, err := os.Stat(t.target)
	if err != nil {
		return err
	}

	if t.PrefixHook != nil {
		err = t.PrefixHook(t.archive)
		if err != nil {
			return err
		}
	}

	// If the target is a file rather than a directory, adjust our initial entry
	// name and target. It will still get just that directory, but want to ensure
	// we don't tar a file named "."
	startFullName := "."
	if !f.IsDir() {
		t.target = filepath.Dir(t.target)
		startFullName = filepath.Join(".", f.Name())
	}

	// walk the directory tree
	if err := t.processEntry(startFullName, f, []string{}); err != nil {
		return err
	}

	if t.SuffixHook != nil {
		err = t.SuffixHook(t.archive)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExcludePath appends a path, file, or pattern relative to the toplevel path to
// be archived that is then excluded from the final archive.
// pathRE is a regex that will be anchored at the start and end then applied to
// the entire filename (full path and basename)
func (t *Tar) ExcludePath(pathRE string) {
	if pathRE != "" {
		re, err := regexp.Compile("^" + pathRE + "$")
		if err != nil {
			return
		}
		t.ignorePaths = append(t.ignorePaths, ignoreInfo{regexp: re, exclude: true, dirOnly: false})
	}
}

// IncludePath appends a path, file, or pattern relative to the toplevel path to
// be archived that is then excluded from the final archive.
// pathRE is a regex that will be anchored at the start and end then applied to
// the entire filename (full path and basename)
func (t *Tar) IncludePath(pathRE string) {
	if pathRE != "" {
		re, err := regexp.Compile("^" + pathRE + "$")
		if err != nil {
			return
		}
		t.ignorePaths = append(t.ignorePaths, ignoreInfo{regexp: re, exclude: false, dirOnly: false})
	}
}

// IncludeRegexp adds a Regexp into the list to consider when selectiong files
// to exclude. Files or directories matching the regexp will _not_ be excluded,
// even if they matched a previous Regexp. Files are only considered a match if
// they match the Regexp and isDir is false.
func (t *Tar) IncludeRegexp(re *regexp.Regexp, dirOnly bool) {
	t.ignorePaths = append(t.ignorePaths, ignoreInfo{regexp: re, exclude: false, dirOnly: dirOnly})
}

// ExcludeRegexp adds a Regexp into the list to consider when selectiong files
// to exclude. Files or directories matching the regexp will be excluded, even
// if they matched a previous Regexp from IncludeRegexp. Files are only
// considered a match if they match the Regexp and isDir is false.
func (t *Tar) ExcludeRegexp(re *regexp.Regexp, dirOnly bool) {
	t.ignorePaths = append(t.ignorePaths, ignoreInfo{regexp: re, exclude: true, dirOnly: dirOnly})
}

func (t *Tar) processDirectory(dir string, dirStack []string) error {
	// get directory entries
	files, err := ioutil.ReadDir(filepath.Join(t.target, dir))
	if err != nil {
		return err
	}

	for _, f := range files {
		fullName := filepath.Join(dir, f.Name())
		if err := t.processEntry(fullName, f, dirStack); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tar) processEntry(fullName string, f os.FileInfo, dirStack []string) error {
	var err error

	// Exclude any files or paths specified by the user.
	if t.shouldBeExcluded(fullName, f.IsDir()) {
		return nil
	}

	// set base header parameters
	header, err := tar.FileInfoHeader(f, "")
	if err != nil {
		return err
	}

	// Correct Windows paths so untar works in stager's container.
	header.Name = path.Join(".", filepath.ToSlash(fullName))

	// handle VirtualPath
	if t.VirtualPath != "" {
		header.Name = path.Join(".", filepath.ToSlash(t.VirtualPath), header.Name)
	}

	// copy uid/gid if Permissions enabled
	if t.IncludeOwners {
		if header.Uid, err = t.OwnerMappingFunc(uidForFileInfo(f)); err != nil {
			return fmt.Errorf("failed to map UID for %q: %v", header.Name, err)
		}
		if header.Gid, err = t.GroupMappingFunc(gidForFileInfo(f)); err != nil {
			return fmt.Errorf("failed to map GID for %q: %v", header.Name, err)
		}
	} else {
		header.Uid = 500
		header.Gid = 500
	}

	// Check for any custom handlers that will process it.
	for _, handler := range t.CustomHandlers {
		bypass, err := handler(filepath.Join(t.target, fullName), f, header)
		if err != nil {
			return err
		}
		if bypass {
			// write the header
			err = t.archive.WriteHeader(header)
			if err != nil {
				return err
			}
			return nil
		}
	}

	// Use built in handlers.
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

		// write the header
		if !t.excludeRootPath(header.Name) {
			err = t.archive.WriteHeader(header)
			if err != nil {
				return err
			}
		}

		// Push the directory to stack
		p, err := filepath.Abs(fullName)
		if err != nil {
			return fmt.Errorf("error getting absolute path for path %q, err='%v'\n", fullName, err)
		}

		// process the directory's entries next
		if err = t.processDirectory(fullName, append(dirStack, p)); err != nil {
			return err
		}

	// symlink handling
	case mode&os.ModeSymlink == os.ModeSymlink:
		// if Permissions is not enabled, force mode back to 0755
		if !t.IncludePermissions {
			header.Mode = 0755
		}

		// read and process the link
		link, err := cleanLinkName(t.target, fullName)
		if err != nil {
			return err
		}

		if t.UserOptions&c_DEREF != 0 {
			// Evaluate the path for the link. This will give us the
			// complete absolute path with all symlinks resolved.
			slink, err := filepath.EvalSymlinks(link)
			if err != nil {
				return fmt.Errorf("error evaluating symlink %q, err='%v'", link, err)
			}

			for _, elem := range dirStack {
				if slink == elem {
					// We don't want to abort if we detect a cycle.
					// Let it continue  without this path element.
					return nil
				}
			}

			// Ok we are not in a circular path. Proceed.
			f, err := os.Stat(slink)
			if err != nil {
				return fmt.Errorf("error getting file stat for %q, err='%v'", slink, err)
			}

			if f.IsDir() {
				// Write the header so that the symlinked directory contents appears
				// under current dir.
				header, err := tar.FileInfoHeader(f, "")
				if err != nil {
					return err
				}
				header.Name = "./" + fullName + "/"

				// write the header
				err = t.archive.WriteHeader(header)
				if err != nil {
					return err
				}

				return t.processDirectory(fullName, append(dirStack, slink))
			} else {
				return t.processEntry(fullName, f, dirStack)
			}

		} else {
			dir := filepath.Dir(fullName)
			// If the link path contains the target path, then convert the link to be
			// relative. This ensures it is properly preserved wherever it is later
			// extracted. If it is a path outside the target, then preserve it as an
			// absolute path.
			if strings.Contains(link, t.target) {
				// remove the targetdir to ensure the link is relative
				link, err = filepath.Rel(filepath.Join(t.target, dir), link)
				if err != nil {
					return err
				}
			}

			header.Linkname = link
			// write the header
			err = t.archive.WriteHeader(header)
			if err != nil {
				return err
			}

		}

	// regular file handling
	case mode&os.ModeType == 0:
		// if Permissions is not enabled, force mode back to 0644
		if !t.IncludePermissions {
			header.Mode = 0644
		}

		// Necessary to ensure files from Windows have +x bit written.
		chmodTarEntry(header)

		// check to see if this is a hard link
		if linkCountForFileInfo(f) > 1 {
			inode := inodeForFileInfo(f)
			if dst, ok := t.hardLinks[inode]; ok {
				// update the header if it is
				header.Typeflag = tar.TypeLink
				header.Linkname = dst
				header.Size = 0
			} else {
				// push it on the list, and continue to write it as a file
				// this is our first time seeing it
				t.hardLinks[inode] = header.Name
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
			data, err := os.Open(filepath.Join(t.target, fullName))
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
		// stat to get devmode
		fi, err := os.Stat(filepath.Join(t.target, fullName))
		header.Devmajor, header.Devminor = osDeviceNumbersForFileInfo(fi)

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
	link, err := os.Readlink(filepath.Join(targetDir, name))
	if err != nil {
		return "", err
	}

	// if the target isn't absolute, make it absolute
	// even if it is absolute, we want to convert it to be relative
	if !filepath.IsAbs(link) {
		link, err = filepath.Abs(filepath.Join(targetDir, dir, link))
		if err != nil {
			return "", err
		}
	}

	// do a quick clean pass
	link = filepath.Clean(link)

	return link, nil
}

// shouldBeExcluded determines if supplied name is contained in the slice of
// files to exclude. ignorePaths are considered in order so that files excluded
// by one criteria can be reincluded by a later one.
func (t *Tar) shouldBeExcluded(name string, isDir bool) bool {
	name = filepath.ToSlash(filepath.Clean(name))
	var exclude bool
	for _, re := range t.ignorePaths {
		if re.regexp.MatchString(name) || re.regexp.MatchString(filepath.Base(name)) {
			if !re.dirOnly || (re.dirOnly && isDir) {
				exclude = re.exclude
			}
		}
	}

	return exclude
}

// excludeRootPath determines if the path is the root path and should be
// excluded.
func (t *Tar) excludeRootPath(headerName string) bool {
	if t.ExcludeRootPath && headerName == "./" {
		return true
	}

	return false
}
