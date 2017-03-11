// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

type MyString string
type foo struct{ aFooString string }
type bar struct{ aBarString string }
type fizz struct{ abuzz buzz }
type buzz struct{ anInt, anInt2, anInt3 int }
type timeFoo struct{ t time.Time }

func TestTestEquals(t *testing.T) {
	m := &MockLogger{}
	m.funcFatalf = func(format string, i ...interface{}) { t.Logf(format, i...) }

	var nilPtr *MockLogger
	strSlice1 := []string{"A", "B", "C"}
	strSlice2 := []string{"A", "B", "C"}
	strMap1 := map[string]int{"A": 1, "B": 2, "C": 3}
	strMap2 := map[string]int{"C": 3, "B": 2, "A": 1}
	myStr1 := MyString("one")
	myStr2 := MyString("one")
	f := func(i int) string { return fmt.Sprint("f", i) }
	g := func(i int) string { return fmt.Sprint("g", i) }
	h := func(i int) int { return i }
	t1 := time.Now()

	// Non failure conditions.
	m.RunTest(t, false, func() { TestEqual(m, nil, nil) })
	m.RunTest(t, false, func() { TestEqual(m, true, true) })
	m.RunTest(t, false, func() { TestEqual(m, int(1), int(1)) })
	m.RunTest(t, false, func() { TestEqual(m, int8(1), int8(1)) })
	m.RunTest(t, false, func() { TestEqual(m, int16(1), int16(1)) })
	m.RunTest(t, false, func() { TestEqual(m, int32(1), int32(1)) })
	m.RunTest(t, false, func() { TestEqual(m, int64(1), int64(1)) })
	m.RunTest(t, false, func() { TestEqual(m, uint(1), uint(1)) })
	m.RunTest(t, false, func() { TestEqual(m, uint8(1), uint8(1)) })
	m.RunTest(t, false, func() { TestEqual(m, uint16(1), uint16(1)) })
	m.RunTest(t, false, func() { TestEqual(m, uint32(1), uint32(1)) })
	m.RunTest(t, false, func() { TestEqual(m, uint64(1), uint64(1)) })
	m.RunTest(t, false, func() { TestEqual(m, float32(1), float32(1)) })
	m.RunTest(t, false, func() { TestEqual(m, float64(1), float64(1)) })
	m.RunTest(t, false, func() { TestEqual(m, "1", "1") })
	m.RunTest(t, false, func() { TestEqual(m, nilPtr, nil) })
	m.RunTest(t, false, func() { TestEqual(m, strSlice1, strSlice2) })
	m.RunTest(t, false, func() { TestEqual(m, strMap1, strMap2) })
	m.RunTest(t, false, func() { TestEqual(m, myStr1, myStr2) })
	m.RunTest(t, false, func() { TestEqual(m, fizz{buzz{1, 2, 3}}, fizz{buzz{1, 2, 3}}) })
	m.RunTest(t, false, func() { TestEqual(m, &fizz{buzz{1, 2, 3}}, &fizz{buzz{1, 2, 3}}) })
	m.RunTest(t, false, func() { TestEqual(m, f, f) })
	m.RunTest(t, false, func() { TestEqual(m, f, g) })
	m.RunTest(t, false, func() { TestEqual(m, t1, t1) })
	m.RunTest(t, false, func() { TestEqual(m, t1, t1.UTC()) })
	m.RunTest(t, false, func() { TestEqual(m, timeFoo{t1}, timeFoo{t1}) })
	m.RunTest(t, false, func() { TestEqual(m, timeFoo{t1}, timeFoo{t1.UTC()}) })

	// Expected failure conditions.
	m.RunTest(t, true, func() { TestEqual(m, &MockLogger{}, nil) })
	m.RunTest(t, true, func() { TestEqual(m, false, true) })
	m.RunTest(t, true, func() { TestEqual(m, int(2), int(1)) })
	m.RunTest(t, true, func() { TestEqual(m, int(1), int8(1)) })
	m.RunTest(t, true, func() { TestEqual(m, int(1), "1") })
	m.RunTest(t, true, func() { TestEqual(m, int8(2), int8(1)) })
	m.RunTest(t, true, func() { TestEqual(m, int16(2), int16(1)) })
	m.RunTest(t, true, func() { TestEqual(m, int32(2), int32(1)) })
	m.RunTest(t, true, func() { TestEqual(m, int64(2), int64(1)) })
	m.RunTest(t, true, func() { TestEqual(m, uint(2), uint(1)) })
	m.RunTest(t, true, func() { TestEqual(m, uint8(2), uint8(1)) })
	m.RunTest(t, true, func() { TestEqual(m, uint16(2), uint16(1)) })
	m.RunTest(t, true, func() { TestEqual(m, uint32(2), uint32(1)) })
	m.RunTest(t, true, func() { TestEqual(m, uint64(2), uint64(1)) })
	m.RunTest(t, true, func() { TestEqual(m, float32(2), float32(1)) })
	m.RunTest(t, true, func() { TestEqual(m, float64(2), float64(1)) })
	m.RunTest(t, true, func() { TestEqual(m, "2", "1") })
	m.RunTest(t, true, func() { TestEqual(m, fizz{buzz{1, 2, 3}}, fizz{buzz{1, 3, 3}}) })
	m.RunTest(t, true, func() { TestEqual(m, &fizz{buzz{1, 2, 3}}, &fizz{buzz{1, 3, 3}}) })
	m.RunTest(t, true, func() { TestEqual(m, fmt.Errorf("foo"), fmt.Errorf("bar")) })
	m.RunTest(t, true, func() { TestEqual(m, bytes.NewBufferString("1"), bytes.NewBufferString("2")) })
	m.RunTest(t, true, func() { TestEqual(m, f, nil) })
	m.RunTest(t, true, func() { TestEqual(m, f, h) })
	m.RunTest(t, true, func() { TestEqual(m, t1, t1.Add(time.Second*1)) })
	m.RunTest(t, true, func() { TestEqual(m, t1, t1.Add(time.Microsecond*1)) })
	m.RunTest(t, true, func() { TestEqual(m, timeFoo{t1}, timeFoo{t1.Add(time.Second * 1)}) })
	m.RunTest(t, true, func() { TestEqual(m, timeFoo{t1}, timeFoo{t1.Add(time.Microsecond * 1)}) })

	strSlice1[0] = "X"
	strMap1["A"] = 3
	m.RunTest(t, true, func() { TestEqual(m, strSlice1, strSlice2) })
	m.RunTest(t, true, func() { TestEqual(m, strMap1, strMap2) })
}

func TestTestNotEquals(t *testing.T) {
	m := &MockLogger{}

	var nilPtr *MockLogger
	strSlice1 := []string{"A", "B", "C"}
	strSlice2 := []string{"A", "B", "C"}
	strMap1 := map[string]int{"A": 1, "B": 2, "C": 3}
	strMap2 := map[string]int{"C": 3, "B": 2, "A": 1}
	f := func(i int) string { return fmt.Sprint("f", i) }
	g := func(i int) string { return fmt.Sprint("g", i) }
	h := func(i int) int { return i }
	t1 := time.Now()

	// Expected failure conditions.
	m.RunTest(t, true, func() { TestNotEqual(m, nil, nil) })
	m.RunTest(t, true, func() { TestNotEqual(m, true, true) })
	m.RunTest(t, true, func() { TestNotEqual(m, int(1), int(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, int8(1), int8(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, int16(1), int16(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, int32(1), int32(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, int64(1), int64(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, uint(1), uint(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, uint8(1), uint8(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, uint16(1), uint16(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, uint32(1), uint32(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, uint64(1), uint64(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, float32(1), float32(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, float64(1), float64(1)) })
	m.RunTest(t, true, func() { TestNotEqual(m, "1", "1") })
	m.RunTest(t, true, func() { TestNotEqual(m, nilPtr, nil) })
	m.RunTest(t, true, func() { TestNotEqual(m, strSlice1, strSlice2) })
	m.RunTest(t, true, func() { TestNotEqual(m, strMap1, strMap2) })
	m.RunTest(t, true, func() { TestNotEqual(m, fizz{buzz{1, 2, 3}}, fizz{buzz{1, 2, 3}}) })
	m.RunTest(t, true, func() { TestNotEqual(m, &fizz{buzz{1, 2, 3}}, &fizz{buzz{1, 2, 3}}) })
	m.RunTest(t, true, func() { TestNotEqual(m, f, f) })
	m.RunTest(t, true, func() { TestNotEqual(m, f, g) })
	m.RunTest(t, true, func() { TestNotEqual(m, t1, t1) })
	m.RunTest(t, true, func() { TestNotEqual(m, t1, t1.UTC()) })

	// Non-failure conditions.
	m.RunTest(t, false, func() { TestNotEqual(m, &MockLogger{}, nil) })
	m.RunTest(t, false, func() { TestNotEqual(m, false, true) })
	m.RunTest(t, false, func() { TestNotEqual(m, int(2), int(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, int(1), int8(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, int(1), "1") })
	m.RunTest(t, false, func() { TestNotEqual(m, int8(2), int8(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, int16(2), int16(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, int32(2), int32(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, int64(2), int64(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, uint(2), uint(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, uint8(2), uint8(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, uint16(2), uint16(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, uint32(2), uint32(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, uint64(2), uint64(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, float32(2), float32(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, float64(2), float64(1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, "2", "1") })
	m.RunTest(t, false, func() { TestNotEqual(m, fizz{buzz{1, 2, 3}}, fizz{buzz{1, 3, 3}}) })
	m.RunTest(t, false, func() { TestNotEqual(m, &fizz{buzz{1, 2, 3}}, &fizz{buzz{1, 3, 3}}) })
	m.RunTest(t, false, func() { TestNotEqual(m, fmt.Errorf("foo"), fmt.Errorf("bar")) })
	m.RunTest(t, false, func() { TestNotEqual(m, bytes.NewBufferString("1"), bytes.NewBufferString("2")) })
	m.RunTest(t, false, func() { TestNotEqual(m, f, nil) })
	m.RunTest(t, false, func() { TestNotEqual(m, f, h) })
	m.RunTest(t, false, func() { TestNotEqual(m, []int{}, []int(nil)) })
	m.RunTest(t, false, func() { TestNotEqual(m, t1, t1.Add(time.Second*1)) })
	m.RunTest(t, false, func() { TestNotEqual(m, t1, t1.Add(time.Microsecond*1)) })

	strSlice1[0] = "X"
	strMap1["A"] = 3
	m.RunTest(t, false, func() { TestNotEqual(m, strSlice1, strSlice2) })
	m.RunTest(t, false, func() { TestNotEqual(m, strMap1, strMap2) })
}
