package proc

import (
	"strings"
	"testing"

	"github.com/apcera/util/testtool"
)

func TestMountPoints(t *testing.T) {
	testtool.StartTest(t)
	defer testtool.FinishTest(t)

	// Test 1: Very basic /proc/mounts file. Ensure that each
	//         field is properly parsed, the order is correct, etc.
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"rootfs1 / rootfs2 rw 0 0",
	}, "\n"))
	if mp, err := MountPoints(); err != nil {
		testtool.Fatalf(t, "Error from MountPoints: %s", err)
	} else if len(mp) != 1 {
		testtool.Fatalf(t, "Bad return value: %#v", mp)
	} else if mp["/"].Dev != "rootfs1" {
		testtool.Fatalf(t, "invalid device: %s", mp["/"].Dev)
	} else if mp["/"].Path != "/" {
		testtool.Fatalf(t, "invalid path: %s", mp["/"].Path)
	} else if mp["/"].Fstype != "rootfs2" {
		testtool.Fatalf(t, "invalid file system type: %s", mp["/"].Fstype)
	} else if mp["/"].Options != "rw" {
		testtool.Fatalf(t, "invalid options: %s", mp["/"].Options)
	} else if mp["/"].Dump != 0 {
		testtool.Fatalf(t, "invalid dump value: %d", mp["/"].Dump)
	} else if mp["/"].Fsck != 0 {
		testtool.Fatalf(t, "invalid fsck value: %d", mp["/"].Fsck)
	}

	// Test 2: Priority, two mounts in the same path. Ensure that
	//         the last listed always wins.
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"bad / bad bad 1 1",
		"rootfs1 / rootfs2 rw 0 0",
	}, "\n"))
	if mp, err := MountPoints(); err != nil {
		testtool.Fatalf(t, "Error from MountPoints: %s", err)
	} else if len(mp) != 1 {
		testtool.Fatalf(t, "Bad return value: %#v", mp)
	} else if mp["/"].Dev != "rootfs1" {
		testtool.Fatalf(t, "invalid device: %s", mp["/"].Dev)
	} else if mp["/"].Path != "/" {
		testtool.Fatalf(t, "invalid path: %s", mp["/"].Path)
	} else if mp["/"].Fstype != "rootfs2" {
		testtool.Fatalf(t, "invalid file system type: %s", mp["/"].Fstype)
	} else if mp["/"].Options != "rw" {
		testtool.Fatalf(t, "invalid options: %s", mp["/"].Options)
	} else if mp["/"].Dump != 0 {
		testtool.Fatalf(t, "invalid dump value: %d", mp["/"].Dump)
	} else if mp["/"].Fsck != 0 {
		testtool.Fatalf(t, "invalid fsck value: %d", mp["/"].Fsck)
	}

	// Test 3: Bad path value (relative or otherwise invalid.)
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev badpath fstype options 0 0",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}

	// Test 4: Bad dump value (not an int)
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev / fstype options bad 0",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}

	// Test 5: Bad dump value (negative)
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev / fstype options -1 0",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}

	// Test 6: Bad dump value (not an int)
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev / fstype options 0 bad",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}

	// Test 7: Bad dump value (negative)
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev / fstype options 0 -1",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}

	// Test 8: Too many columns.
	MountProcFile = testtool.WriteTempFile(t, strings.Join([]string{
		"dev / fstype options 0 0 extra",
	}, "\n"))
	if _, err := MountPoints(); err == nil {
		testtool.Fatalf(t, "Expected an error from MountPoints()")
	}
}
