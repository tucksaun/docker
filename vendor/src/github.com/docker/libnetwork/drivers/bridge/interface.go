package bridge

import (
	"net"

	"github.com/vishvananda/netlink"
)

// Interface models the bridge network device.
type bridgeInterface struct {
	Link        netlink.Link
	bridgeIPv4  *net.IPNet
	bridgeIPv6  *net.IPNet
	gatewayIPv4 net.IP
	gatewayIPv6 net.IP
}

// exists indicates if the existing bridge interface exists on the system.
func (i *bridgeInterface) exists() bool {
	return i.Link != nil
}

// addresses returns a single IPv4 address and all IPv6 addresses for the
// bridge interface.
func (i *bridgeInterface) addresses() (netlink.Addr, []netlink.Addr, error) {
	v4addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V4)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	v6addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V6)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	if len(v4addr) == 0 {
		return netlink.Addr{}, v6addr, nil
	}
	return v4addr[0], v6addr, nil
}
