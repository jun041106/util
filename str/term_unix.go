// Copyright 2012 Apcera Inc. All rights reserved.

// +build linux

package str

// Above may add freebsd after we test it there.
// When you change the above make sure to change term_other.go

import (
	"os"
	"syscall"
	"unsafe"
)

// May move to util/file? For now is here because it's the only
// one used by colors.
func IsTerminal(file *os.File) bool {
    var termios syscall.Termios
    _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, file.Fd(),
        uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
    return (err == 0)
}

