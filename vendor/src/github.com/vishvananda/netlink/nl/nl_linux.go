// Package nl has low level primitives for making Netlink calls.
package nl

import (
	"syscall"
	"unsafe"
)

func ParseRouteAttr(b []byte) ([]syscall.NetlinkRouteAttr, error) {
	var attrs []syscall.NetlinkRouteAttr
	for len(b) >= syscall.SizeofRtAttr {
		a, vbuf, alen, err := netlinkRouteAttrAndValue(b)
		if err != nil {
			return nil, err
		}
		ra := syscall.NetlinkRouteAttr{Attr: *a, Value: vbuf[:int(a.Len)-syscall.SizeofRtAttr]}
		attrs = append(attrs, ra)
		b = b[alen:]
	}
	return attrs, nil
}

func netlinkRouteAttrAndValue(b []byte) (*syscall.RtAttr, []byte, int, error) {
	a := (*syscall.RtAttr)(unsafe.Pointer(&b[0]))
	if int(a.Len) < syscall.SizeofRtAttr || int(a.Len) > len(b) {
		return nil, nil, 0, syscall.EINVAL
	}
	return a, b[syscall.SizeofRtAttr:], rtaAlignOf(int(a.Len)), nil
}
