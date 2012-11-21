// Copyright 2012 Apcera Inc. All rights reserved.

package util

import (
	"errors"
	"fmt"
)

// Error with file and line information.
// You use TracedError same way as any other regular error object:
//    import "github.com/apcera/util"
//
//    func SomeFunc(param int) error {
//        if param > 5 {
//            return util.Errorf("Invalid parameter: %d", param)  
//        }
//        ...
//    }
//
//    func main() {
//        ...
//        err := someFunc(20)
//        if err != nil {
//            log.Fatalf("returned error: %v\n", err)
//        }
//        ...
//    } 
//
// The output of such example will be something like:
//
//   returned error: company/somepkg/server.go:155: invalid parameter: 20
// 
type TracedError struct {
	// Full path of the source file.
	Path		string
	
	// Last two, or less, elements of the path to source file.
	// This path is used by the Error() function.	
	ShortPath	string
	
	// Name of the source file that generated the error.
	File		string
	
	// Line number that generated the error.
	Line		int
	
	// The generated error.
	Err 		error	
}

// Returns text of the original error.
func (e *TracedError) ErrorString() string {
	return e.Err.Error()	
}

// Returns text of the error in the form:
//     "file:line: <original error text>"
func (e *TracedError) Error() string {
	return fmt.Sprintf("%s/%s:%d: %v", e.ShortPath, e.File, e.Line, e.Err)	
}

func newError(path, file string, line int) *TracedError {
	e := &TracedError{}
	e.Path = path
	e.File = file
	e.Line = line
	if len(path) > 0 {
		subpath, path1 := splitPath(path)
		if len(subpath) > 0 {
			_, path2 := splitPath(subpath)
			e.ShortPath = path2 + "/" + path1
		} else {
			e.ShortPath = path
		}
		
	}
	return e
}

// Create TracedError with specified error message.
// Notice that *TracedError implements error interface and can be returned
// by a function as "error" type.
func Error(err string) *TracedError {
	e := newError(GetCallerFileLine())
	e.Err = errors.New(err)
	return e
}

// Create TracedError using formatted sprintf to build error message.
// Notice that *TracedError implements error interface and can be returned
// by a function as "error" type.
func Errorf(format string, params ...interface{}) *TracedError {
	e := newError(GetCallerFileLine())
	e.Err = fmt.Errorf(format, params...)
	return e
}

// Create TracedError using specified original error.
// Notice that *TracedError implements error interface and can be returned
// by a function as "error" type.
func Errore(err error) *TracedError {
	e := newError(GetCallerFileLine())
	e.Err = err
	return e
}


