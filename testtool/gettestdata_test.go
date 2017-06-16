package testtool

import (
	"regexp"
	"testing"
)

func TestGetTestData(t *testing.T) {

	check := func(t *testing.T, name string, exp, got TestData) {
		// each string field of TestData is a regex to match
		check := func(exp, got string) {
			ok, err := regexp.MatchString(exp, got)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !ok {
				t.Fatalf("%s: got %s, expected %s", name, got, exp)
			}
		}

		check(exp.File, got.File)
		check(exp.Package, got.Package)
		check(exp.TestName, got.TestName)
		check(exp.PackageDir, got.PackageDir)
	}

	check(t,
		"self",
		TestData{
			File:       `^.*/util/testtool/gettestdata_test\.go$`,
			Package:    "^testtool$",
			TestName:   "^TestGetTestData$",
			PackageDir: "^.*/util/testtool$",
		},
		*GetTestData(t))

	func() {
		check(t,
			"closure",
			TestData{
				File:       `^.*/util/testtool/gettestdata_test\.go$`,
				Package:    "^testtool$",
				TestName:   `^TestGetTestData\.func[^\.]+$`,
				PackageDir: "^.*/util/testtool$",
			},
			*GetTestData(t))
	}()

	t.Run("An éßø†é®îç subtest", func(t *testing.T) {
		check(t,
			"subtest",
			TestData{
				File:       `^.*/util/testtool/gettestdata_test\.go$`,
				Package:    "^testtool$",
				TestName:   `^TestGetTestData\.func[^\.]+$`,
				PackageDir: "^.*/util/testtool$",
			},
			*GetTestData(t))
	})
}
