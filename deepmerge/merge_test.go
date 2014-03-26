// Copyright 2014 Apcera Inc. All rights reserved.

package deepmerge

import (
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestDeepMergeBasic(t *testing.T) {
	dst := map[string]interface{}{
		"one":   1,
		"three": 3,
	}
	src := map[string]interface{}{
		"two":  "2",
		"four": 4,
	}
	expected := map[string]interface{}{
		"one":   1,
		"two":   "2",
		"three": 3,
		"four":  4,
	}
	Merge(dst, src)
	tt.TestEqual(t, dst, expected)
}

func TestDeepMergeOverwriteSlice(t *testing.T) {
	dst := map[string]interface{}{
		"one":       1,
		"groceries": []string{"eggs", "milk", "cereal"},
		"people":    []string{"John", "Tom", "Joe"},
	}
	src := map[string]interface{}{
		"groceries": []interface{}{"bread", "cereal", "juice"},
	}
	expected := map[string]interface{}{
		"one":       1,
		"groceries": []interface{}{"bread", "cereal", "juice"},
		"people":    []string{"John", "Tom", "Joe"},
	}
	Merge(dst, src)
	tt.TestEqual(t, dst, expected)
}

func TestDeepMergeRecursiveMap(t *testing.T) {
	dst := map[string]interface{}{
		"settings": map[string]interface{}{
			"internal": map[string]interface{}{
				"foo": "bar",
				"baz": []interface{}{"arg", "agg", "ugh"},
			},
			"external": map[string]interface{}{
				"path":     "/",
				"approved": false,
				"number":   123,
			},
		},
		"domain": "example.com",
	}
	src := map[string]interface{}{
		"admin":  "John",
		"domain": "example.org",
		"settings": map[string]interface{}{
			"internal": map[string]interface{}{
				"foo": "buf",
			},
			"external": map[string]interface{}{
				"path":     []interface{}{"/v1", "/v2"},
				"approved": true,
				"allowed":  false,
			},
			"wildcard": map[string]interface{}{
				"destination": "home",
				"source":      "work",
			},
		},
	}
	expected := map[string]interface{}{
		"domain": "example.org",
		"admin":  "John",
		"settings": map[string]interface{}{
			"internal": map[string]interface{}{
				"foo": "buf",
				"baz": []interface{}{"arg", "agg", "ugh"},
			},
			"external": map[string]interface{}{
				"path":     []interface{}{"/v1", "/v2"},
				"approved": true,
				"allowed":  false,
				"number":   123,
			},
			"wildcard": map[string]interface{}{
				"destination": "home",
				"source":      "work",
			},
		},
	}
	Merge(dst, src)
	tt.TestEqual(t, dst, expected)

}
