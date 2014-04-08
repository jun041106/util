// Copyright 2014 Apcera Inc. All rights reserved.

package iprange

import (
	"net"
	"testing"

	tt "github.com/apcera/util/testtool"
)

func TestIPRangeParseBasicStringIPv4(t *testing.T) {
	//
	// success
	//

	// 192.168.1.1-100
	ipr, err := ParseIPRange("192.168.1.1-100")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.1.100")

	// 192.168.1.1-100/25
	ipr, err = ParseIPRange("192.168.1.1-100/25")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.1.100")
	oneBits, _ := ipr.Mask.Size()
	tt.TestEqual(t, oneBits, 25)

	// 192.168.1.1
	ipr, err = ParseIPRange("192.168.1.1")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.1.1")

	// 192.168.1.1-2.1
	ipr, err = ParseIPRange("192.168.1.1-2.1")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.2.1")

	// 192.168.1.1-2.1/22
	ipr, err = ParseIPRange("192.168.1.1-2.1/22")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.2.1")
	oneBits, _ = ipr.Mask.Size()
	tt.TestEqual(t, oneBits, 22)

	// 192.168.1.1/24
	ipr, err = ParseIPRange("192.168.1.1/24")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr.Start.String(), "192.168.1.1")
	tt.TestEqual(t, ipr.End.String(), "192.168.1.1")
	oneBits, _ = ipr.Mask.Size()
	tt.TestEqual(t, oneBits, 24)

	//
	// errors
	//

	// 192.168.1.100-1
	_, err = ParseIPRange("192.168.1.100-1")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "the end of the range cannot be less than the start of the range")

	// 192.168.1.1/zz
	_, err = ParseIPRange("192.168.1.1/zz")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "failed to parse the network mask: strconv.ParseInt: parsing \"zz\": invalid syntax")

	// 192.168.1.1-255/32
	_, err = ParseIPRange("192.168.1.1-255/32")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "the provided IP ranges are not within the provided network mask")

	// 192.168.1.100-1-2
	_, err = ParseIPRange("192.168.1.100-1-2")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "unexpected number of IPs specified in the provided string")

	// 192.168.1.100-150/24/24
	_, err = ParseIPRange("192.168.1.100-150/24/24")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "expected only one '/' within the provided string")
}

func TestIPRangeOverlap(t *testing.T) {
	ipr1, err := ParseIPRange("192.168.1.1-100")
	tt.TestExpectSuccess(t, err)
	ipr2, err := ParseIPRange("192.168.1.101-200")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), false)

	ipr2, err = ParseIPRange("192.168.1.100-200")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), true)

	ipr2, err = ParseIPRange("192.168.1.50-55")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), true)

	ipr2, err = ParseIPRange("192.168.1.1")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), true)

	ipr1, err = ParseIPRange("192.168.1.200-210")
	tt.TestExpectSuccess(t, err)
	ipr2, err = ParseIPRange("192.168.1.100-150")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), false)

	ipr2, err = ParseIPRange("192.168.1.100-250")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), true)

	ipr1, err = ParseIPRange("192.168.1.1-255/24")
	tt.TestExpectSuccess(t, err)
	ipr2, err = ParseIPRange("192.168.0.1-3.255/22")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, ipr1.Overlaps(ipr2), true)
}

func TestIPRangeContains(t *testing.T) {
	ipr1, err := ParseIPRange("192.168.1.10-50")
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.1")), false)
	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.9")), false)
	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.51")), false)
	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.2.10")), false)

	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.10")), true)
	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.20")), true)
	tt.TestEqual(t, ipr1.Contains(net.ParseIP("192.168.1.50")), true)
}
