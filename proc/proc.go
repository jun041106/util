// Copyright 2013 Apcera Inc. All rights reserved.

package proc

import (
	"fmt"
	"strconv"
)

type MountPoint struct {
	Dev     string
	Path    string
	Fstype  string
	Options string
	Dump    int
	Fsck    int
}

// This is the location of the proc mount point. Typically this is only
// modified by unit testing.
var MountProcFile string = "/proc/mounts"

// Reads through /proc/mounts and returns the data associated with the mount
// points as a list of MountPoint structures.
func MountPoints() (map[string]*MountPoint, error) {
	mp := make(map[string]*MountPoint, 0)
	var current *MountPoint
	err := ParseSimpleProcFile(
		MountProcFile,
		nil,
		func(line int, index int, elm string) error {
			switch index {
			case 0:
				current = new(MountPoint)
				current.Dev = elm
			case 1:
				if len(elm) > 0 && elm[0] != '/' {
					return fmt.Errorf(
						"Invalid path on lin %d of file %s: %s",
						line, MountProcFile, elm)
				}
				current.Path = elm
				mp[elm] = current
			case 2:
				current.Fstype = elm
			case 3:
				current.Options = elm
			case 4:
				n, err := strconv.ParseUint(elm, 10, 32)
				if err != nil {
					return fmt.Errorf(
						"Error parsing column %d on line %d of file %s: %s",
						index, line, MountProcFile, elm)
				}
				current.Dump = int(n)
			case 5:
				n, err := strconv.ParseUint(elm, 10, 32)
				if err != nil {
					return fmt.Errorf(
						"Error parsing column %d on line %d of file %s: %s",
						index, line, MountProcFile, elm)
				}
				current.Fsck = int(n)
			default:
				return fmt.Errorf(
					"Too many colums on line %d of file %s",
					line, MountProcFile)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return mp, nil
}
