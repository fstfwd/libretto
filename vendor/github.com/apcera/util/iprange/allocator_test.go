// Copyright 2014 Apcera Inc. All rights reserved.

package iprange

import (
	"fmt"
	"net"
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestNewAllocator(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)

	alloc := NewAllocator(ipr)
	tt.TestEqual(t, alloc.size, int64(10))
	tt.TestEqual(t, alloc.remaining, int64(10))
}

func TestAllocateSingleIPRange(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)
	tt.TestEqual(t, alloc.size, int64(1))
	tt.TestEqual(t, alloc.Remaining(), int64(1))

	// get the first one
	ip := alloc.Allocate()
	tt.TestEqual(t, ip.String(), "192.168.1.10")
	tt.TestEqual(t, ipr.Contains(ip), true)
	tt.TestEqual(t, alloc.Remaining(), int64(0))

	// no more left
	ip = alloc.Allocate()
	tt.TestEqual(t, ip, nil)
	tt.TestEqual(t, alloc.Remaining(), int64(0))
}

func TestAllocate(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)

	// get the first one
	ip := alloc.Allocate()
	tt.TestEqual(t, ipr.Contains(ip), true)
	tt.TestEqual(t, alloc.Remaining(), int64(9))

	// consume the others
	for i := 0; i < 8; i++ {
		ip = alloc.Allocate()
		tt.TestEqual(t, ipr.Contains(ip), true, fmt.Sprintf("%s was not within the range", ip.String()))
	}
	tt.TestEqual(t, alloc.Remaining(), int64(1))

	// last ip
	ip = alloc.Allocate()
	tt.TestEqual(t, ipr.Contains(ip), true)
	tt.TestEqual(t, alloc.Remaining(), int64(0))

	// no more left
	ip = alloc.Allocate()
	tt.TestEqual(t, ip, nil)
	tt.TestEqual(t, alloc.Remaining(), int64(0))
}

func TestReserve(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)

	// reserve an IP
	reservedIP := net.ParseIP("192.168.1.11")
	alloc.Reserve(reservedIP)
	tt.TestEqual(t, alloc.remaining, int64(9))
	tt.TestEqual(t, len(alloc.reserved), 1)

	// consume everything and ensure we don't get that IP
	for {
		if alloc.Remaining() == 0 {
			break
		}

		ip := alloc.Allocate()
		tt.TestNotEqual(t, ip, nil)
		tt.TestNotEqual(t, ip, reservedIP)
	}
}

func TestReserveOutOfRange(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)

	// reserve an IP
	reservedIP := net.ParseIP("10.0.0.1")
	alloc.Reserve(reservedIP)
	tt.TestEqual(t, alloc.remaining, int64(10))
	tt.TestEqual(t, len(alloc.reserved), 0)
}

func TestSubtract(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)
	tt.TestEqual(t, alloc.remaining, int64(10))

	// create a smaller range within the same one
	ipr2, err := ParseIPRange("192.168.1.10-14")
	tt.TestExpectSuccess(t, err)

	// subtract it
	alloc.Subtract(ipr2)

	// validate it
	tt.TestEqual(t, alloc.remaining, int64(5))
	tt.TestEqual(t, len(alloc.reserved), 5)

	// consume everything and ensure we don't get an IP in the second range.
	for {
		if alloc.Remaining() == 0 {
			break
		}

		ip := alloc.Allocate()
		tt.TestNotEqual(t, ip, nil)
		tt.TestEqual(t, ipr2.Contains(ip), false)
	}
}

func TestRelease(t *testing.T) {
	ipr, err := ParseIPRange("192.168.1.10-19")
	tt.TestExpectSuccess(t, err)
	alloc := NewAllocator(ipr)

	// test releasing when empty
	alloc.Release(net.ParseIP("192.168.1.11"))
	tt.TestEqual(t, alloc.remaining, int64(10))

	// consume everything
	for {
		if alloc.Remaining() == 0 {
			break
		}
		ip := alloc.Allocate()
		tt.TestNotEqual(t, ip, nil)
	}

	// release an IP
	tt.TestEqual(t, alloc.remaining, int64(0))
	alloc.Release(net.ParseIP("192.168.1.11"))
	tt.TestEqual(t, alloc.remaining, int64(1))

	// allocate one more and should get that one
	tt.TestEqual(t, alloc.Allocate().String(), "192.168.1.11")
	tt.TestEqual(t, alloc.remaining, int64(0))
}
