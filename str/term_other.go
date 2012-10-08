// Copyright 2012 Apcera Inc. All rights reserved.

// +build !linux,!darwin

package str

import (
	"os"
)

// For now if not Linux or Darwin, say no.
func IsTerminal(file *os.File) bool {
    return false
}









