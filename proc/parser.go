// Copyright 2013 Apcera Inc. All rights reserved.

package proc

import (
	"io/ioutil"
	"os"
)

// Parses the given file into various elements. This function assumes basic
// white space semantics (' ' and '\t' for column splitting, and '\n' for
// row splitting.
func ParseSimpleProcFile(
	filename string,
	lf func(index int, line string) error,
	ef func(line int, index int, elm string) error) error {
	//
	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()
	contentsBytes, err := ioutil.ReadAll(fd)
	if err != nil {
		return err
	}

	if lf == nil {
		lf = func(index int, line string) error { return nil }
	}
	if ef == nil {
		ef = func(line int, index int, elm string) error { return nil }
	}

	lineindex := 0
	linestart := 0
	elmstart := 0
	elmindex := 0
	contents := string(contentsBytes)
	for i, r := range contents {
		if r != ' ' && r != '\t' && r != '\n' {
			continue
		}

		if i == elmstart {
			// Line starts with white space, do not include it in the
			// element that we will pass to the function. Push the
			// start forward one element.
			elmstart = i + 1
		} else {
			err := ef(lineindex, elmindex, contents[elmstart:i])
			if err != nil {
				return err
			}
			elmstart = i + 1
			elmindex += 1
		}

		// Return condition.
		if r == '\n' {
			if err := lf(lineindex, contents[linestart:i]); err != nil {
				return err
			}
			elmstart = i + 1
			elmindex = 0
			lineindex += 1
			linestart = i + 1
		}
	}

	// Process the last element if needed.
	if elmstart < len(contents) {
		err := ef(lineindex, elmindex, contents[elmstart:len(contents)])
		if err != nil {
			return err
		}
	}
	if linestart < len(contents) {
		i := len(contents)
		if err := lf(lineindex, contents[linestart:i]); err != nil {
			return err
		}
	}

	return nil
}
