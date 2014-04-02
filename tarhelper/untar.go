// Copyright 2012-2013 Apcera Inc. All rights reserved.

package tarhelper

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
)

// The type of compression that this archive will be us
type Compression int

const (
	NONE   = Compression(0)
	BZIP2  = Compression(1)
	GZIP   = Compression(2)
	DETECT = Compression(999)
)

type resolvedLink struct {
	src string
	dst string
}

// Untar manages state of a TAR archive to be extracted.
type Untar struct {

	// The directory that the files will be extracted into. This will
	// be the root for all paths contained within the tar file.
	target string

	// The source reader.
	source io.Reader

	// A list of currently resolved links. This is used to ensure when creating
	// a file that follows through a symlink, we create the file relative to the
	// location of the AbsoluteRoot.
	resolvedLinks []resolvedLink

	// The AbsoluteRoot is intended to be the root of the target and allows us
	// to create files that follow through links that are absolute paths, but
	// ensure the file is created relative to the AbsoluteRoot and not the root
	// on the host system.
	AbsoluteRoot string

	// The Compression being used in this tar.
	Compression Compression

	// The archive/tar reader that we will use to extract each
	// element from the tar file. This will be set when Extract()
	// is called.
	archive *tar.Reader

	// Set to true if extraction should attempt to preserve
	// permissions as recorded in the tar file. If this is false then
	// files will be created with a default of 755 for directories and 644
	// for files.
	PreservePermissions bool

	// Set to true if extraction should attempt to restore owners of files
	// and directories from the archive.  Any Uid/Gid over 500 will be set
	// to the MappedUserID/MappedGroupID setting.  If this is set to false
	// it will default to all files going to the MappedUserID/MappedGroupID.
	PreserveOwners bool

	// The default UID to set files with an owner over 500 to. If PreserveOwners
	// is false, this will be the UID assigned for all files in the archive.
	// This defaults to the UID of the current running user.
	MappedUserID int

	// The default GID to set files with an owner over 500 to. If PreserveOwners
	// is false, this will be the GID assigned for all files in the archive.
	// This defaults to the GID of the current running user.
	MappedGroupID int

	// IncludedPermissionMask is combined with the uploaded file mask as a way to
	// ensure a base level of permissions for all objects.
	IncludedPermissionMask os.FileMode
}

// NewUntar returns an Untar to use to extract the contents of r into targetDir.
// Extraction is handled by Extract().
func NewUntar(r io.Reader, targetDir string) *Untar {
	u := &Untar{
		source:              r,
		target:              targetDir,
		PreservePermissions: true,
		PreserveOwners:      false,
		AbsoluteRoot:        "/",
		resolvedLinks:       make([]resolvedLink, 0),
	}

	// loop up the current user for mapping of files
	// only do it if err != nil
	if usr, err := user.Current(); err != nil {
		if usr == nil {
			Log.V(1).Info("user.Current() returned a nil user")
			u.MappedUserID = 500
			u.MappedGroupID = 500
		} else {
			if u.MappedUserID, err = strconv.Atoi(usr.Uid); err != nil {
				u.MappedUserID = 500
			}
			if u.MappedGroupID, err = strconv.Atoi(usr.Gid); err != nil {
				u.MappedGroupID = 500
			}
		}
	} else {
		u.MappedUserID = 500
		u.MappedGroupID = 500
	}

	return u
}

// Extract unpacks the tar reader that was passed into New(). This is
// broken out from new to give the caller time to set various
// settings in the Untar object.
func (u *Untar) Extract() error {
	// Since this component does not have a context to work within
	// all of its logging is done as V(1).Info so that they do not
	// end up spamming logs. Its assumed that the upstream
	// caller will log these errors properly.
	Log.V(1).Info("Extracting tar file.")

	// check for detect mode before the main setup, we'll change compression
	// to the intended type and setup a new reader to re-read the header
	if u.Compression == DETECT {
		// setup a buffered reader
		br := bufio.NewReader(u.source)

		// read the first 2 bytes
		data, err := br.Peek(2)
		if err != nil {
			return err
		}

		switch {
		case data[0] == 0x1f && data[1] == 0x8b:
			// gzip
			u.Compression = GZIP

		case data[0] == 0x42 && data[1] == 0x5a:
			// bzip2
			u.Compression = BZIP2

		default:
			// assume none
			u.Compression = NONE
		}

		// set the main source to the buffered reader
		u.source = br
	}

	// Create a TarReader that wraps the proper io.Reader object
	// the implements the expected compression for this file.
	switch u.Compression {
	case NONE:
		u.archive = tar.NewReader(u.source)
	case BZIP2:
		source := bzip2.NewReader(u.source)
		u.archive = tar.NewReader(source)
	case GZIP:
		source, err := gzip.NewReader(u.source)
		if err != nil {
			// TODO: offload this into Process since it forces a header read.
			Log.V(1).Infof("Error creating gzip reader: %s", err)
			return nil
		}
		u.archive = tar.NewReader(source)
	default:
		Log.Errorf("Unknown compression type (%v)", u.Compression)
		return fmt.Errorf("Unknown compression type (%v)", u.Compression)
	}

	for {
		header, err := u.archive.Next()
		if err == io.EOF {
			// EOF, ok, break to return
			Log.V(1).Info("Success.")
			break
		}
		if err != nil {
			// See note on logging above.
			Log.V(1).Infof("Error reading tar element header: %s", err)
			return err
		}

		err = u.processEntry(header)
		if err != nil {
			// See note on logging above.
			Log.V(1).Infof("Error reading tar element contents: %s", err)
			return err
		}
	}

	return nil
}

// Checks the security of the given name. Anything that looks
// fishy will be rejected.
func checkName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("No name given for tar element.")
	}
	comp := strings.Split(name, string(os.PathSeparator))
	if len(comp) > 0 && comp[0] == "" {
		return fmt.Errorf("No absolute paths allowed.")
	}
	for i, c := range comp {
		switch {
		case c == "" && i != len(comp)-1:
			// don't allow an empty name, unless it is the last element... handles
			// cases where we may have "./" come in as the name
			return fmt.Errorf("Empty name in file path.")
		case c == "..":
			return fmt.Errorf("Double dots not allowed in path.")
		}
	}
	return nil
}

// Checks the security of the given link name. Anything that looks fishy
// will be rejected.
func checkLinkName(dest, src, targetBase string) error {
	if len(dest) == 0 {
		return fmt.Errorf("No name given for tar element.")
	}
	return nil
}

// Processes a single header/body combination from the tar
// archive being processed in Extract() above.
func (u *Untar) processEntry(header *tar.Header) error {
	// Check the security of the name being given to us by tar.
	// If the name contains any bad things then we force
	// an error in order to protect ourselves.
	if err := checkName(header.Name); err != nil {
		Log.V(1).Infof("Security error in the tar file: %s", err)
		return err
	}

	name := path.Join(u.target, header.Name)

	// resolve the destination and then reset the name based on the resolution
	destDir, err := u.resolveDestination(path.Dir(name))
	name = path.Join(destDir, path.Base(name))
	if err != nil {
		return err
	}

	// look at the type to see how we want to remove existing entries
	switch {
	case header.Typeflag == tar.TypeDir:
		// if we are extracting a directory, we want to see if the directory
		// already exists... if it exists but isn't a directory, we need
		// to remove it
		fi, _ := os.Stat(name)
		if fi != nil {
			if !fi.IsDir() {
				os.RemoveAll(name)
			}
		}
	default:
		os.RemoveAll(name)
	}

	// handle individual types
	switch {
	case header.Typeflag == tar.TypeDir:
		// Handle directories
		// don't return error if it already exists
		mode := os.FileMode(0755)
		if u.PreservePermissions {
			mode = os.FileMode(header.Mode) | u.IncludedPermissionMask
		}

		// create the directory
		err := os.MkdirAll(name, mode)
		if err != nil {
			return err
		}

	case header.Typeflag == tar.TypeSymlink:
		// Handle symlinks
		err := checkLinkName(header.Linkname, name, u.target)
		if err != nil {
			return err
		}

		// have seen links to themselves
		if name == header.Linkname {
			break
		}

		// make the link
		if err := os.Symlink(header.Linkname, name); err != nil {
			return err
		}

	case header.Typeflag == tar.TypeLink:
		// handle creation of hard links
		if err := checkLinkName(header.Linkname, name, u.target); err != nil {
			return err
		}

		// find the full path, need to ensure it exists
		link := path.Clean(path.Join(u.target, header.Linkname))

		// do the link... no permissions or owners, those carry over
		if err := os.Link(link, name); err != nil {
			return err
		}

	case header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA:
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		// determine the mode to use
		mode := os.FileMode(0644)
		if u.PreservePermissions {
			mode = os.FileMode(header.Mode) | u.IncludedPermissionMask
		}

		// open the file
		f, err := os.OpenFile(name, flags, mode)
		if err != nil {
			return err
		}
		defer f.Close()

		// SETUID/SETGID needs to be defered...
		// The standard chown call is after handling the files, since we want to
		// just have it one place, and after the file exists.  However, chown
		// will clear the setuid/setgid bit on a file.
		if header.Mode&c_ISUID != 0 {
			defer lazyChmod(name, os.ModeSetuid)
		}
		if header.Mode&c_ISGID != 0 {
			defer lazyChmod(name, os.ModeSetgid)
		}

		// copy the contents
		n, err := io.Copy(f, u.archive)
		if err != nil {
			Log.V(1).Infof("Error while copying file %s: %s", name, err)
			return err
		} else if n != header.Size {
			Log.V(1).Infof("Short write while copying file %s", name)
			return fmt.Errorf("Short write while copying file %s", name)
		}

	case header.Typeflag == tar.TypeBlock || header.Typeflag == tar.TypeChar:
		// determine how to OR the mode
		devmode := uint32(0)
		switch header.Typeflag {
		case tar.TypeChar:
			devmode = syscall.S_IFCHR
		case tar.TypeBlock:
			devmode = syscall.S_IFBLK
		}

		// determine the mode to use
		mode := os.FileMode(0644)
		if u.PreservePermissions {
			mode = os.FileMode(header.Mode) | u.IncludedPermissionMask
		}

		// syscall to mknod
		dev := makedev(header.Devmajor, header.Devminor)
		syscall.Umask(0000)
		if err := syscall.Mknod(name, devmode|uint32(mode), dev); err != nil {
			return err
		}

	default:
		Log.V(1).Infof("Unknown type in tar file: %d", header.Typeflag)
		return fmt.Errorf("Unrecognized type: %d", header.Typeflag)
	}

	// process the uid/gid ownership
	uid := u.MappedUserID
	gid := u.MappedGroupID
	if header.Uid < 500 && u.PreserveOwners {
		uid = header.Uid
	}
	if header.Gid < 500 && u.PreserveOwners {
		gid = header.Gid
	}

	// apply it
	switch header.Typeflag {
	case tar.TypeSymlink:
		os.Lchown(name, uid, gid)
	case tar.TypeLink:
		// don't chown on hard links or symlinks. doing this also removes setuid
		// from mode and the hard link will already pick up the same owner
	default:
		os.Chown(name, uid, gid)
	}

	return nil
}

func (u *Untar) resolveDestination(name string) (string, error) {
	pathParts := strings.Split(name, string(os.PathSeparator))

	// this prefix is used to know if we're absolute paths or not later when
	// rejoining
	prefix := "." + string(os.PathSeparator)
	if path.IsAbs(name) {
		prefix = string(os.PathSeparator)
	}

	// walk the path parts to find at what point the resolvedLinks deviates
	i := 0
	for i, _ = range pathParts {
		if (i < len(u.resolvedLinks)) && pathParts[i] == u.resolvedLinks[i].src {
			continue
		}
		break
	}

	// truncate the slice to only the matching pieces
	u.resolvedLinks = u.resolvedLinks[0:i]

	// special handling for an empty array...
	// normally it begins with the previous dest, but if it is empty we need to
	// start with resolving the first path piece
	if len(u.resolvedLinks) == 0 {
		dst, err := u.convertToDestination(path.Join(prefix, pathParts[i]))
		if err != nil {
			return "", err
		}

		u.resolvedLinks = append(
			u.resolvedLinks,
			resolvedLink{src: pathParts[i], dst: dst})
		i++
	}

	// build up the resolution for the rest of the pieces
	for j := i; j < len(pathParts); j++ {
		testPath := path.Join(
			prefix,
			u.resolvedLinks[len(u.resolvedLinks)-1].dst,
			pathParts[j])
		dst, err := u.convertToDestination(testPath)
		if err != nil {
			return "", err
		}

		u.resolvedLinks = append(
			u.resolvedLinks,
			resolvedLink{src: pathParts[j], dst: dst})
	}

	// the last entry is the full resolution
	return u.resolvedLinks[len(u.resolvedLinks)-1].dst, nil
}

func (u *Untar) convertToDestination(dir string) (string, error) {
	// Lstat the current element to see if it is a symlink
	if dir == "" {
		dir = "."
	}
	lstat, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}

	// check symlink mode
	if lstat.Mode()&os.ModeSymlink == os.ModeSymlink {
		// it is a symlink, now we want to read it and store the dest
		link, err := os.Readlink(dir)
		if err != nil {
			return "", err
		}

		// if the path is absolute, we want it based on the AbsoluteRoot
		if path.IsAbs(link) {
			link = path.Join(u.AbsoluteRoot, ".", link)
			link = path.Clean(link)
		} else {
			// clean up the path to be a more complete dest from the target
			link = path.Join(path.Dir(dir), ".", link)
			link = path.Clean(link)
		}

		// return the link
		return link, nil
	}

	// not a symlink, so return the dir
	return dir, nil
}

func lazyChmod(name string, m os.FileMode) {
	if fi, err := os.Stat(name); err == nil {
		if err := os.Chmod(name, fi.Mode()|m); err != nil {
			Log.Warningf("Failed to chmod '%s': %v", name, err)
		}
	} else {
		Log.Warningf("Failed to stat '%s': %v", name, err)
	}
}
