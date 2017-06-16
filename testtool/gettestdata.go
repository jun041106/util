package testtool

import (
	"path"
	"runtime"
	"strings"
	"testing"
)

type TestData struct {
	File       string
	Package    string
	TestName   string
	PackageDir string
	Line       int
}

// GetTestData goes through the call stack of the current goroutine and creates
// a ordered set of strings for the files, function names, and line numbers then
// adds this data to the error message.
func GetTestData(tb testing.TB) *TestData {
	var pcs [20]uintptr
	pcCount := runtime.Callers(2, pcs[:])
	pcCount -= 2

	scanned := []string{}
	for _, pc := range pcs[0:pcCount] {
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc - 1)
		dir, packageFunction := path.Split(f.Name())

		ss := strings.SplitN(packageFunction, ".", 2)
		pkg := ""
		function := ""
		switch len(ss) {
		case 1:
			function = ss[0]
		case 2:
			pkg = ss[0]
			function = ss[1]
		}
		dir = path.Join(dir, pkg)

		scanned = append(scanned, function)
		if strings.HasPrefix(function, "Test") ||
			strings.HasPrefix(function, "Benchmark") {

			return &TestData{
				File:       file,
				Line:       line,
				TestName:   function,
				Package:    pkg,
				PackageDir: dir,
			}
		}
	}

	tb.Fatalf("No TestXXX or BenchmarkXXX function name found on the call stack of:\n%s",
		strings.Join(scanned, "\n\t"))
	return nil
}
