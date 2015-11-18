package netlink

import (
	"fmt"
	"syscall"
)

const (
	XFRM_PROTO_ROUTE2    Proto = syscall.IPPROTO_ROUTING
	XFRM_PROTO_ESP       Proto = syscall.IPPROTO_ESP
	XFRM_PROTO_AH        Proto = syscall.IPPROTO_AH
	XFRM_PROTO_HAO       Proto = syscall.IPPROTO_DSTOPTS
	XFRM_PROTO_COMP      Proto = syscall.IPPROTO_COMP
	XFRM_PROTO_IPSEC_ANY Proto = syscall.IPPROTO_RAW
)

func (p Proto) String() string {
	switch p {
	case XFRM_PROTO_ROUTE2:
		return "route2"
	case XFRM_PROTO_ESP:
		return "esp"
	case XFRM_PROTO_AH:
		return "ah"
	case XFRM_PROTO_HAO:
		return "hao"
	case XFRM_PROTO_COMP:
		return "comp"
	case XFRM_PROTO_IPSEC_ANY:
		return "ipsec-any"
	}
	return fmt.Sprintf("%d", p)
}