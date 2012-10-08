// Copyright 2012 Apcera Inc. All rights reserved.

// +build !linux

package str

import (
	"os"
)

// For now if not Linux or Darwin, say no.
func IsTerminal(file *os.File) bool {
    return (file == os.Stdout || file == os.Stderr)
}









