// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

var typeTime = reflect.TypeOf(time.Now())

// -----------------------------------------------------------------------
// Equality tests.
// -----------------------------------------------------------------------

// Returns true if the value is nil. Interfaces can actually NOT be nil since
// they have a type attached to them, even if the interface value is nil so
// we check both cases in this function.
func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	// If the value is a Kind which can store nil then we actually
	// check it, otherwise the IsNil() call can panic.
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Func:
	case reflect.Interface:
	case reflect.Map:
	case reflect.Ptr:
	case reflect.Slice:
	default:
		return false
	}
	return v.IsNil()
}

func TestExpectNonNil(t Logger, i interface{}, msg ...string) {
	if haveNil := isNil(i); haveNil {
		Fatalf(t, "Expected non-nil value, got nil. %s", msg)
	}
}

func TestTrue(t Logger, ans bool) {
	if !ans {
		Fatalf(t, "Expected a true value.")
	}
}

func TestFalse(t Logger, ans bool) {
	if ans {
		Fatalf(t, "Expected a false value.")
	}
}

func TestMatch(t Logger, have string, r *regexp.Regexp) {
	if !r.MatchString(have) {
		Fatalf(t, "Expected %s to match regexp %v.", have, r)
	}
}

func TestNotMatch(t Logger, have string, r *regexp.Regexp) {
	if r.MatchString(have) {
		Fatalf(t, "Expected %s to not match regexp %v.", have, r)
	}
}

func TestEqual(t Logger, have, want interface{}, msg ...string) {
	haveNil := isNil(have)
	wantNil := isNil(want)
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if haveNil && wantNil {
		return
	} else if haveNil && !wantNil {
		Fatalf(t, "%sExpected non nil, got nil.", reason)
	} else if !haveNil && wantNil {
		Fatalf(t, "%sExpected nil, got non nil: %#v", reason, have)
	}
	haveValue := reflect.ValueOf(have)
	wantValue := reflect.ValueOf(want)
	r := deepValueEqual("", haveValue, wantValue, make(map[uintptr]*visit))
	if len(r) != 0 {
		Fatalf(t, "Not Equal%s\n%s", reason, strings.Join(r, "\n"))
	}
}

func TestNotEqual(t Logger, have, want interface{}, msg ...string) {
	haveNil := isNil(have)
	wantNil := isNil(want)
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if haveNil && wantNil {
		Fatalf(t, "%sEquality not expected, have=nil", reason)
	} else if haveNil || wantNil {
		return
	}
	haveValue := reflect.ValueOf(have)
	wantValue := reflect.ValueOf(want)
	r := deepValueEqual("", haveValue, wantValue, make(map[uintptr]*visit))
	if len(r) == 0 {
		Fatalf(t,
			"Equality not expected%s\nhave: %#v", reason, have)
	}
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
func deepValueEqual(description string, have, want reflect.Value, visited map[uintptr]*visit) (diffs []string) {
	if !want.IsValid() && !have.IsValid() {
		return nil
	} else if !want.IsValid() && have.IsValid() {
		// This is rare, not sure how to document this better.
		return []string{
			fmt.Sprintf("%s: wanted an invalid object", description),
			fmt.Sprintf(" have: %#v", have),
			fmt.Sprintf(" want: %#v", want),
		}
	} else if want.IsValid() && !have.IsValid() {
		// This is rare, not sure how to document this better.
		return []string{
			fmt.Sprintf("%s: wanted an valid object", description),
			fmt.Sprintf(" have: %#v", have),
			fmt.Sprintf(" want: %#v", want),
		}
	}

	wantType := want.Type()
	if wantType != have.Type() {
		return []string{fmt.Sprintf(
			"%s: Not the same type, have: '%s', want: '%s'",
			description, have.Type(), want.Type())}
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
			return []string{}
		}

		// ... or already seen
		h := 17*addr1 + addr2
		seen := visited[h]
		for p := seen; p != nil; p = p.next {
			if p.a1 == addr1 && p.a2 == addr2 && p.typ == wantType {
				return []string{}
			}
		}

		// Remember for later.
		visited[h] = &visit{addr1, addr2, wantType, seen}
	}

	// Checks to see if one value is nil, while the other is not.
	checkNil := func() bool {
		if want.IsNil() && !have.IsNil() {
			diffs = append(diffs, fmt.Sprintf("%s: not equal.", description))
			diffs = append(diffs, fmt.Sprintf(" have: %#v", have))
			diffs = append(diffs, " want: nil")
			return true
		} else if !want.IsNil() && have.IsNil() {
			diffs = append(diffs, fmt.Sprintf("%s: not equal.", description))
			diffs = append(diffs, " have: nil")
			diffs = append(diffs, fmt.Sprintf(" want: %#v", want))
			return true
		}
		return false
	}

	// Checks to see that the lengths of both objects are equal.
	checkLen := func() bool {
		if want.Len() != have.Len() {
			diffs = append(diffs, fmt.Sprintf(
				"%s: (len(have): %d, len(want): %d)",
				description, have.Len(), want.Len()))
			diffs = append(diffs, fmt.Sprintf(" have: %#v", have.Interface()))
			diffs = append(diffs, fmt.Sprintf(" want: %#v", want.Interface()))
			return true
		}
		return false
	}

	switch want.Kind() {
	case reflect.Array:
		if !checkLen() {
			for i := 0; i < want.Len(); i++ {
				newdiffs := deepValueEqual(
					fmt.Sprintf("%s[%d]", description, i),
					have.Index(i), want.Index(i), visited)
				diffs = append(diffs, newdiffs...)
			}
		}
	case reflect.Slice:
		if !checkNil() && !checkLen() {
			for i := 0; i < want.Len(); i++ {
				newdiffs := deepValueEqual(
					fmt.Sprintf("%s[%d]", description, i),
					have.Index(i), want.Index(i), visited)
				diffs = append(diffs, newdiffs...)
			}
		}
	case reflect.Interface:
		if !checkNil() {
			newdiffs := deepValueEqual(description, have.Elem(), want.Elem(), visited)
			diffs = append(diffs, newdiffs...)
		}
	case reflect.Ptr:
		newdiffs := deepValueEqual(description, have.Elem(), want.Elem(), visited)
		diffs = append(diffs, newdiffs...)
	case reflect.Struct:
		// Custom time comparison to simulate time.Equal rather than DeepEqual.
		if wantType == typeTime {
			return timesEqual(description, have, want)
		}
		for i, n := 0, want.NumField(); i < n; i++ {
			f := wantType.Field(i)
			// Make sure that we don't print a strange error if the
			// first object given to us is a struct.
			if description == "" {
				newdiffs := deepValueEqual(
					f.Name, have.Field(i), want.Field(i), visited)
				diffs = append(diffs, newdiffs...)
			} else {
				newdiffs := deepValueEqual(
					fmt.Sprintf("%s.%s", description, f.Name),
					have.Field(i), want.Field(i), visited)
				diffs = append(diffs, newdiffs...)
			}
		}
	case reflect.Map:
		if !checkNil() {
			// Check that the keys are present in both maps.
			for _, k := range want.MapKeys() {
				if !have.MapIndex(k).IsValid() {
					// Add the error.
					diffs = append(diffs, fmt.Sprintf(
						"%sExpected key [%q] is missing.", description, k))
					diffs = append(diffs, " have: not present")
					diffs = append(diffs,
						fmt.Sprintf(" want: %#v", want.MapIndex(k).Interface()))
					continue
				}
				newdiffs := deepValueEqual(
					fmt.Sprintf("%s[%q] ", description, k),
					have.MapIndex(k), want.MapIndex(k), visited)
				diffs = append(diffs, newdiffs...)
			}
			for _, k := range have.MapKeys() {
				if !want.MapIndex(k).IsValid() {
					// Add the error.
					diffs = append(diffs, fmt.Sprintf("%sUnexpected key [%q].", description, k))
					diffs = append(diffs, fmt.Sprintf(" have: %#v", have.MapIndex(k).Interface()))
					diffs = append(diffs, " want: not present")
				}
			}
		}
	case reflect.Func:
		// Can't do better than this; the Types are the same so the method
		// signatures match.
		checkNil()
	case reflect.String:
		// We know the underlying type is a string so calling String()
		// will return the underlying value. Trying to call Interface()
		// and assert to a string will panic.
		s1 := have.String()
		s2 := want.String()
		if len(s1) != len(s2) {
			return []string{
				fmt.Sprintf(
					"%s: len(have) %d != len(want) %d.",
					description, len(s1), len(s2)),
				fmt.Sprintf(" have: %#v", s1),
				fmt.Sprintf(" want: %#v", s2),
			}
		}
		for i := range s1 {
			if s1[i] != s2[i] {
				return []string{
					fmt.Sprintf(
						"%s: difference at index %d.",
						description, i),
					fmt.Sprintf(" have: %#v", s1),
					fmt.Sprintf(" want: %#v", s2),
				}
			}
		}
	case reflect.Bool:
		v1, v2 := have.Bool(), want.Bool()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %v, want %v", description, v1, v2)}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v1, v2 := have.Int(), want.Int()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %v, want %v", description, v1, v2)}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v1, v2 := have.Uint(), want.Uint()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %d, want %d", description, v1, v2)}
		}
	case reflect.Uintptr:
		v1, v2 := have.Uint(), want.Uint()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %#x, want %#x", description, v1, v2)}
		}
	case reflect.Float32, reflect.Float64:
		v1, v2 := have.Float(), want.Float()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %v, want %v", description, v1, v2)}
		}
	case reflect.Complex64, reflect.Complex128:
		v1, v2 := have.Complex(), want.Complex()
		if v1 != v2 {
			return []string{fmt.Sprintf("%s: have %v, want %v", description, v1, v2)}
		}
	default:
		// Chan, UnsafePointer, Invalid.
		if !reflect.DeepEqual(have, want) {
			return []string{fmt.Sprintf("%s: have %v, want %v", description, have, want)}
		}
	}

	return diffs
}

// timesEqual simulates using time.Equal() rather than reflect.DeepEqual so that
// moments in time are compared rather than including locations which only add
// information for presentation.
func timesEqual(description string, have, want reflect.Value) (diffs []string) {
	// Special case when CanInterface is true. Recent change in fmt will call
	// this before printing so we can't do the string comparison of the internal
	// fields.
	if have.CanInterface() {
		t1, t2 := have.Interface().(time.Time), want.Interface().(time.Time)
		if !t1.Equal(t2) {
			return []string{"Not equal (using time.Equal):",
				fmt.Sprintf(" have: %v", t1),
				fmt.Sprintf(" want: %v", t2),
			}
		}
		return
	}

	haveParts := strings.Split(strings.Trim(fmt.Sprintf("%v", have), "{}"), " ")
	wantParts := strings.Split(strings.Trim(fmt.Sprintf("%v", want), "{}"), " ")
	if len(haveParts) != len(wantParts) || len(haveParts) != 3 {
		return []string{"Unexpected time.Time format; can't compare:",
			fmt.Sprintf(" have: %#v", have),
			fmt.Sprintf(" want: %#v", want),
		}
	}
	if haveParts[0] != wantParts[0] {
		diffs = append(diffs, []string{
			fmt.Sprintf("%s: time.Time seconds not equal", description),
			fmt.Sprintf(" have: %v", haveParts[0]),
			fmt.Sprintf(" want: %v", wantParts[0]),
		}...)
	}
	if haveParts[1] != wantParts[1] {
		diffs = append(diffs, []string{
			fmt.Sprintf("%s: time.Time nanoseconds not equal", description),
			fmt.Sprintf(" have: %v", haveParts[1]),
			fmt.Sprintf(" want: %v", wantParts[1]),
		}...)
	}
	return diffs
}
