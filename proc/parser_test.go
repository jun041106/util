// Copyright 2013 Apcera Inc. All rights reserved.

package proc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/apcera/util/testtool"
)

func TestParseSimpleProcFile(t *testing.T) {
	testtool.StartTest(t)
	defer testtool.FinishTest(t)

	// Test 1: Success.
	lines := []string{
		"aelm0 aelm1\taelm2",
		" belm0  belm1\t belm2\t\t\t",
		"",
		"delm0"}
	f := testtool.WriteTempFile(t, strings.Join(lines, "\n"))
	err := ParseSimpleProcFile(
		f,
		func(index int, line string) error {
			if index > len(lines) {
				t.Fatalf("Too many lines read: %d", index)
			} else if line != lines[index] {
				t.Fatalf("Invalid line read: %s", line)
			}
			return nil
		},
		func(line int, index int, elm string) error {
			switch {
			case line == 0 && index == 0 && elm == "aelm0":
			case line == 0 && index == 1 && elm == "aelm1":
			case line == 0 && index == 2 && elm == "aelm2":
			case line == 1 && index == 0 && elm == "belm0":
			case line == 1 && index == 1 && elm == "belm1":
			case line == 1 && index == 2 && elm == "belm2":
			case line == 3 && index == 0 && elm == "delm0":
			default:
				t.Fatalf("Unknown element read: %d, %d, %s", line, index, elm)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("Unexpected error from ParseSimpleProcFile()")
	}

	// Test 2: No function defined. This should be successful.
	err = ParseSimpleProcFile(f, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error from ParseSimpleProcFile()")
	}

	// Test 3: ef returns an error.
	err = ParseSimpleProcFile(
		f,
		func(index int, line string) error {
			return fmt.Errorf("error.")
		},
		nil)
	if err == nil {
		t.Fatalf("Expected error not returned.")
	}

	// Test 4: lf returns an error.
	err = ParseSimpleProcFile(
		f,
		nil,
		func(line int, index int, elm string) error {
			return fmt.Errorf("error.")
		})
	if err == nil {
		t.Fatalf("Expected error not returned.")
	}

	// Test 6: last case lf operation.
	err = ParseSimpleProcFile(
		f,
		func(index int, line string) error {
			if line == "delm0" {
				return fmt.Errorf("error")
			}
			return nil
		},
		nil)

	// Test 5: last case lf operation.
	err = ParseSimpleProcFile(
		f,
		nil,
		func(line int, index int, elm string) error {
			if elm == "delm0" {
				return fmt.Errorf("error")
			}
			return nil
		})
}
