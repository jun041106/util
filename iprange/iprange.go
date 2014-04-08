// Copyright 2014 Apcera Inc. All rights reserved.

package iprange

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// IPRange is used to represent a range of IP addresses, such as
// "192.168.1.1-100". It can be used to parse string representations and check
// if other provided IPs are within the given range. It can also be used with
// other utilies to handle allocation of IPs from the provided range.
type IPRange struct {
	Start net.IP
	End   net.IP
	Mask  net.IPMask
}

// ParseIPRange creates an IPRange object based on the provided string
// representing the range. The string for a range is in the form of
// "192.168.1.1-100", to specify a range of IPs from 192.168.1.1 to
// 192.168.1.100. The string can also contain a network mask, such as
// "192.168.1.1-100/24". Strings can span over multiple octets, such as
// "192.168.1.1-2.1", and a range can also be just a single IP. An error will be
// returned if it fails to parse the IPs, if the end IP isn't after the start
// IP, and if a network mask is given, it will error if the mask is in valid, or
// the range does not fall within the bounds of the provided mask.
func ParseIPRange(s string) (*IPRange, error) {
	ipr := &IPRange{}

	// check if the string contains a network mask
	if strings.Contains(s, "/") {
		p := strings.Split(s, "/")
		if len(p) != 2 {
			return nil, fmt.Errorf("expected only one '/' within the provided string")
		}
		s = p[0]
		maskBits, err := strconv.Atoi(p[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse the network mask: %v", err)
		}
		ipr.Mask = net.CIDRMask(maskBits, 32)
	}

	// parse out the dash between the start-end IP portions
	ips := strings.Split(s, "-")
	if len(ips) > 2 {
		return nil, fmt.Errorf("unexpected number of IPs specified in the provided string")
	}
	ipr.Start = net.ParseIP(ips[0])
	if len(ips) > 1 {
		ipr.End = net.ParseIP(spliceIP(ips[0], ips[1]))
	} else {
		ipr.End = ipr.Start
	}

	// ensure the end is after the start
	if bytes.Compare([]byte(ipr.End), []byte(ipr.Start)) < 0 {
		return nil, fmt.Errorf("the end of the range cannot be less than the start of the range")
	}

	// if a subnet was given, then ensure the IPs are within it
	if len(ipr.Mask) > 0 {
		ipnet := net.IPNet{
			IP:   ipr.Start,
			Mask: ipr.Mask,
		}
		if !ipnet.Contains(ipr.End) {
			return nil, fmt.Errorf("the provided IP ranges are not within the provided network mask")
		}
	}

	return ipr, nil
}

// Contains returns whether or not the given IP address is within the specified
// IPRange.
func (ipr *IPRange) Contains(ip net.IP) bool {
	// if ip is less than start, return false
	if bytes.Compare([]byte(ip), []byte(ipr.Start)) < 0 {
		return false
	}
	// return true if ip is less than or equal to end
	return bytes.Compare([]byte(ip), []byte(ipr.End)) <= 0
}

// Overlaps checks whether another IPRange instance has an overlap in IPs with
// the current range. If will return true if there is any cross section between
// the two ranges.
func (ipr *IPRange) Overlaps(o *IPRange) bool {
	// if the start of o is less than our start, we need to make sure the end of o
	// is less than our start
	if bytes.Compare([]byte(o.Start), []byte(ipr.Start)) < 0 {
		return bytes.Compare([]byte(o.End), []byte(ipr.Start)) >= 0
	}
	// if the start of o is greater than our end, then no overlap
	if bytes.Compare([]byte(o.Start), []byte(ipr.End)) > 0 {
		return false
	}
	// otherwise, their start is within our range, and thus there is overlap
	return true
}

// FIXME this only handles IPv4 at the moment
func spliceIP(baseIP, partialIP string) string {
	baseParts := strings.Split(baseIP, ".")
	partialParts := strings.Split(partialIP, ".")
	finalParts := append(baseParts[:(len(baseParts)-len(partialParts))], partialParts...)
	return strings.Join(finalParts, ".")
}
