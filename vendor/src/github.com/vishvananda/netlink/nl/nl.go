// Package nl has low level primitives for making Netlink calls.
package nl

import (
	"net"
	"syscall"
)

const (
	// Family type definitions
	FAMILY_ALL = syscall.AF_UNSPEC
	FAMILY_V4  = syscall.AF_INET
	FAMILY_V6  = syscall.AF_INET6
)

var nextSeqNr uint32

// GetIPFamily returns the family type of a net.IP.
func GetIPFamily(ip net.IP) int {
	if len(ip) <= net.IPv4len {
		return FAMILY_V4
	}
	if ip.To4() != nil {
		return FAMILY_V4
	}
	return FAMILY_V6
}
