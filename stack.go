// Copyright 2012 Apcera Inc. All rights reserved.

package util

import (
	"fmt"
	"path"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"
)

func splitPath(full string) (string, string) {
	n := -1
	if runtime.GOOS != "windows" {
		n = strings.LastIndex(full, "/")
	} else {
		// I do not know nor tested if Go handles "/" on Windows,
		// which is a legitimate path separator there, so just do manually.
		for i := len(full)-1; i >= 0; i-- {
			if full[i] == '/' || full[i] == '\\' {
				n = i
				break
			}
		}
	}
	if n >= 0 {
		return full[:n], full[n+1:]
	}
	return "", full			  
}

func getFileLine(skip int) (string, string, int) {

	_, fullName, line, ok := runtime.Caller(1+skip)
	if ok {
		path, file := splitPath(fullName) 
		return path, file, line
	}
	
	return "<unknown>", "<unknown>", 0
}

// Returns path to source file, source file name and the line number of the
// caller. 
func GetCallerFileLine() (path string, file string, line int) {
	return getFileLine(2)
}

// Returns path to source file, source file name and the line number of the
// current line. 
func GetCurrentFileLine() (path string, file string, line int) {
	return getFileLine(1)
}

// Print stack trace.
// Stack trace is printed using fmt.Print() and lists only the files,
// line number and the function name of each call frame, in a tabular
// format.
func PrintStackTrace() {
	// Skip 1 to to not print this function
	printSkipStackTrace(1)
}

// Print stack trace skipping first "skip" elements in the stack.
// This does not panic if too many call frames were skipped but
// the printed stack trace will be empty.
func PrintSkipStackTrace(skip int) {
	// Skip 1 to to not print this function
	printSkipStackTrace(1+skip)
}

// Custom normalization.
// This replaces all '\' with '/' on Windows and also accounts for some
// windows perks. On Windows this does not remove the last / from "C:/"
// but removes last / by calling path.Clean() in all other cases.
// It also shortens the package paths to last two path elements. It is
// usually enough.
func normalizePath(p string) string {
	n := len(p)
	if n == 0 {
		return p
	}
	if strings.ToLower(runtime.GOOS) == "windows" {
		if n == 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
			return p[:2] + "/"
		}
		// Do not replace first two \\ 
		if n >= 2 && p[0] == '\\' && p[1] == '\\' {
			p = `\\` + strings.Replace(p[2:], "\\", "/", -1)
		} else {
			p = strings.Replace(p, "\\", "/", -1)
		}
	}
	s := path.Clean(p)
	// extract last path elements
	parts := strings.Split(s, "/")
	n = len(parts)
	if n > 3 {
		return parts[n-3] + "/" + parts[n-2] + "/" + parts[n-1]
	}

	return s
}

func printSkipStackTrace(extraSkips int) {

	var maxLen, printed int

	start := 1 + extraSkips // skip myself and extra user said to skip

	count := start
	for i := start; ; i++ {
		pc, file, _, ok := runtime.Caller(i)
		if !ok {
			break
		}
		count++
		file = normalizePath(file)
		if rc := utf8.RuneCountInString(file); maxLen < rc {
			maxLen = rc
		}
		if f := runtime.FuncForPC(pc); f == nil || f.Name() == "main.main" {
			break
		}
	}

	format := fmt.Sprintf("%s%d%s",
		"  at %-",
		(maxLen + 5), // 6 is ':' and up to 9999 line num
		"s  in %s()\n")

	fmt.Printf("*** Stack Trace ***\n")

	for i := start; i < count; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		file = normalizePath(file)
		f := runtime.FuncForPC(pc)
		if f == nil {
			break
		}
		fstr := file + ":" + strconv.Itoa(line)

		sn := f.Name()
		parts := strings.Split(sn, "/")
		n := len(parts)
		if n > 2 {
			sn = parts[n-2] + "/" + parts[n-1]
		}

		fmt.Printf(format, fstr, sn)
		printed++
	}

	if printed == 0 {
		fmt.Printf("  <empty stack trace> (skipped too many?)\n\n")
	} else {
		fmt.Printf("\n")
	}
}
