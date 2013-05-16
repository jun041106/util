// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"reflect"
	"testing"
)

type visit struct {
	a1   uintptr
	a2   uintptr
	typ  reflect.Type
	next *visit
}

// This is ripped directly from golang 1.1 and modified in order to
// make this a little more unit test friendly.
// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueEqual(
	t *testing.T, description string, v1, v2 reflect.Value, depth int,
	visited map[uintptr]*visit,
) {
	if !v1.IsValid() || !v2.IsValid() {
		if v1.IsValid() != v2.IsValid() {
			Fatalf(t, "%s: Validity of both is not the same.", description)
		}
	}
	if v1.Type() != v2.Type() {
		Fatalf(t, "%s: Not the same type.", description)
	}

	if v1.CanAddr() && v2.CanAddr() {
		addr1 := v1.UnsafeAddr()
		addr2 := v2.UnsafeAddr()
		if addr1 > addr2 {
			// Canonicalize order to reduce number of entries in visited.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are identical ...
		if addr1 == addr2 {
			return
		}

		// ... or already seen
		h := 17*addr1 + addr2
		seen := visited[h]
		typ := v1.Type()
		for p := seen; p != nil; p = p.next {
			if p.a1 == addr1 && p.a2 == addr2 && p.typ == typ {
				return
			}
		}

		// Remember for later.
		visited[h] = &visit{addr1, addr2, typ, seen}
	}

	switch v1.Kind() {
	case reflect.Array:
		if v1.Len() != v2.Len() {
			Fatalf(t, "%s: lengths are not equal.", description)
		}
		for i := 0; i < v1.Len(); i++ {
			deepValueEqual(
				t, fmt.Sprintf("%s [%d]: ", i, description),
				v1.Index(i), v2.Index(i), depth+1, visited)
		}
	case reflect.Slice:
		if v1.IsNil() && !v2.IsNil() {
			Fatalf(t, "%s: expect nil, got something else.", description)
		} else if !v1.IsNil() && v2.IsNil() {
			Fatalf(t, "%s: expect non nil, got nil.", description)
		}
		if v1.Len() != v2.Len() {
			Fatalf(
				t, "%s: expected length %d, got length %s", description,
				v1.Len(), v2.Len())
		}
		for i := 0; i < v1.Len(); i++ {
			deepValueEqual(
				t, fmt.Sprintf("%s[%d]", description, i),
				v1.Index(i), v2.Index(i), depth+1, visited)
		}
	case reflect.Interface:
		if v1.IsNil() && !v2.IsNil() {
			Fatalf(t, "%s: expect nil, got something else.", description)
		} else if !v1.IsNil() && v2.IsNil() {
			Fatalf(t, "%s: expect non nil, got nil.", description)
		}
		deepValueEqual(
			t, description, v1.Elem(), v2.Elem(), depth+1, visited)
	case reflect.Ptr:
		deepValueEqual(
			t, description, v1.Elem(), v2.Elem(), depth+1, visited)
	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			field1 := v1.Type().Field(i)
			field2 := v2.Type().Field(i)
			if field1.Name != field2.Name {
				Fatalf(
					t, "%s Field names do not match: %s != %s",
					description, field1.Name, field2.Name)
			}
			// Make sure that we don't print a strange error if the
			// first object given to us is a struct.
			if description == "" {
				deepValueEqual(
					t, field1.Name, v1.Field(i), v2.Field(i), depth+1, visited)
			} else {
				deepValueEqual(
					t, fmt.Sprintf("%s.%s ", description, field1.Name),
					v1.Field(i), v2.Field(i), depth+1, visited)
			}
		}
	case reflect.Map:
		if v1.IsNil() && !v2.IsNil() {
			Fatalf(t, "%s: expect nil, got something else.", description)
		} else if !v1.IsNil() && v2.IsNil() {
			Fatalf(t, "%s: expect non nil, got nil.", description)
		}
		for _, k := range v1.MapKeys() {
			deepValueEqual(
				t, fmt.Sprintf("%s[%s] ", description, k),
				v1.MapIndex(k), v2.MapIndex(k), depth+1, visited)
		}
	case reflect.Func:
		if v1.IsNil() && !v2.IsNil() {
			Fatalf(t, "%s: expect nil, got something else.", description)
		} else if !v1.IsNil() && v2.IsNil() {
			Fatalf(t, "%s: expect non nil, got nil.", description)
		}
		// Can't do better than this:
	default:
		// Normal equality suffices
		if !reflect.DeepEqual(v1.Interface(), v2.Interface()) {
			Fatalf(t, "%s: not equal.", description)
		}
	}
}
