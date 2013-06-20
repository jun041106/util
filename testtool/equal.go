// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"reflect"
	"testing"
)

// -----------------------------------------------------------------------
// Equality tests.
// -----------------------------------------------------------------------

func TestEqual(t *testing.T, have, want interface{}) {
	if have == nil && want != nil {
		Fatalf(t, "Expected non nil, got nil.")
	} else if have != nil && want == nil {
		Fatalf(t, "Expected nil, got non nil.")
	}
	haveValue := reflect.ValueOf(have)
	wantValue := reflect.ValueOf(want)
	deepValueEqual(t, "", haveValue, wantValue, make(map[uintptr]*visit))
}

// ---------
// Internals
// ---------

// Tracks access to specific pointers so we do not recurse.
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
	t *testing.T, description string, have, want reflect.Value,
	visited map[uintptr]*visit,
) {
	if !want.IsValid() && !have.IsValid() {
		return
	} else if !want.IsValid() && have.IsValid() {
		// This is rare, not sure how to document this better.
		Fatalf(t, "%s: have invalid object.", description)
	} else if want.IsValid() && !have.IsValid() {
		// This is rare, not sure how to document this better.
		Fatalf(t, "%s: wanted a valid object.", description)
	} else if want.Type() != have.Type() {
		Fatalf(
			t, "%s: Not the same type, have: '%s', want: '%s'",
			description, have.Type(), want.Type())
	}

	if want.CanAddr() && have.CanAddr() {
		addr1 := want.UnsafeAddr()
		addr2 := have.UnsafeAddr()
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
		typ := want.Type()
		for p := seen; p != nil; p = p.next {
			if p.a1 == addr1 && p.a2 == addr2 && p.typ == typ {
				return
			}
		}

		// Remember for later.
		visited[h] = &visit{addr1, addr2, typ, seen}
	}

	// Checks to see if one value is nil, while the other is not.
	checkNil := func() {
		if want.IsNil() && !have.IsNil() {
			Fatalf(
				t, "%s: not equal.\nhave: %s\nwant: nil.",
				description, have.Interface())
		} else if !want.IsNil() && have.IsNil() {
			Fatalf(
				t, "%s: not equal.\nhave: nil\nwant: %s",
				description, want.Interface())
		}
	}

	// Checks to see that the lengths of both objects are equal.
	checkLen := func() {
		if want.Len() != have.Len() {
			Fatalf(
				t, "%s: (len(have): %d, len(want): %d)\nhave: %s\nwant: %s",
				description, have.Len(), want.Len(),
				have.Interface(), want.Interface())
		}
	}

	switch want.Kind() {
	case reflect.Array:
		checkLen()
		for i := 0; i < want.Len(); i++ {
			deepValueEqual(
				t, fmt.Sprintf("%s[%d]", i, description),
				want.Index(i), have.Index(i), visited)
		}

	case reflect.Slice:
		checkNil()
		checkLen()
		for i := 0; i < want.Len(); i++ {
			deepValueEqual(
				t, fmt.Sprintf("%s[%d]", description, i),
				want.Index(i), have.Index(i), visited)
		}

	case reflect.Interface:
		checkNil()
		deepValueEqual(
			t, description, want.Elem(), have.Elem(), visited)

	case reflect.Ptr:
		deepValueEqual(
			t, description, want.Elem(), have.Elem(), visited)

	case reflect.Struct:
		for i, n := 0, want.NumField(); i < n; i++ {
			name := want.Type().Field(i).Name
			// Make sure that we don't print a strange error if the
			// first object given to us is a struct.
			if description == "" {
				deepValueEqual(
					t, name, want.Field(i), have.Field(i), visited)
			} else {
				deepValueEqual(
					t, fmt.Sprintf("%s.%s", description, name),
					want.Field(i), have.Field(i), visited)
			}
		}

	case reflect.Map:
		checkNil()
		checkLen()
		for _, k := range want.MapKeys() {
			deepValueEqual(
				t, fmt.Sprintf("%s[%s] ", description, k),
				want.MapIndex(k), have.MapIndex(k), visited)
		}

	case reflect.Func:
		// Can't do better than this:
		checkNil()

	case reflect.String:
		s1 := have.Interface().(string)
		s2 := want.Interface().(string)
		if len(s1) != len(s2) {
			Fatalf(t,
				"%s: len(have) %d != len(want) %d.\nhave: %s\nwant: %s\n",
				description, len(s1), len(s2), s1, s2)
		}
		for i := range s1 {
			if s1[i] != s2[i] {
				Fatalf(t,
					"%s: difference at index %d.\nhave: %s\nwant: %s\n",
					description, i, s1, s2)
			}
		}

	default:
		// Specific low level types:
		switch want.Interface().(type) {
		case bool:
			s1 := have.Interface().(bool)
			s2 := want.Interface().(bool)
			if s1 != s2 {
				Fatalf(t, "%s: have %t, want %t", description, s1, s2)
			}
		case int:
			s1 := have.Interface().(int)
			s2 := want.Interface().(int)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case int8:
			s1 := have.Interface().(int8)
			s2 := want.Interface().(int8)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case int16:
			s1 := have.Interface().(int16)
			s2 := want.Interface().(int16)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case int32:
			s1 := have.Interface().(int32)
			s2 := want.Interface().(int32)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case int64:
			s1 := have.Interface().(int64)
			s2 := want.Interface().(int64)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uint:
			s1 := have.Interface().(uint)
			s2 := want.Interface().(uint)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uint8:
			s1 := have.Interface().(uint8)
			s2 := want.Interface().(uint8)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uint16:
			s1 := have.Interface().(uint16)
			s2 := want.Interface().(uint16)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uint32:
			s1 := have.Interface().(uint32)
			s2 := want.Interface().(uint32)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uint64:
			s1 := have.Interface().(uint64)
			s2 := want.Interface().(uint64)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case uintptr:
			s1 := have.Interface().(uintptr)
			s2 := want.Interface().(uintptr)
			if s1 != s2 {
				Fatalf(t, "%s: have %d, want %d", description, s1, s2)
			}
		case float32:
			s1 := have.Interface().(float32)
			s2 := want.Interface().(float32)
			if s1 != s2 {
				Fatalf(t, "%s: have %f, want %f", description, s1, s2)
			}
		case float64:
			s1 := have.Interface().(float64)
			s2 := want.Interface().(float64)
			if s1 != s2 {
				Fatalf(t, "%s: have %f, want %f", description, s1, s2)
			}
		default:
			// Normal equality suffices
			if !reflect.DeepEqual(want.Interface(), have.Interface()) {
				Fatalf(
					t, "%s: not equal.\nhave: %s\nwant: %s",
					description, have, want)
			}
		}
	}
}
