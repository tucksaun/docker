package netlink

import (
	"fmt"
	"net"
)

// Neigh represents a link layer neighbor from netlink.
type Neigh struct {
	LinkIndex    int
	Family       int
	State        int
	Type         int
	Flags        int
	IP           net.IP
	HardwareAddr net.HardwareAddr
}

type Ndmsg struct {
	Family uint8
	Index  uint32
	State  uint16
	Flags  uint8
	Type   uint8
}

// String returns $ip/$hwaddr $label
func (neigh *Neigh) String() string {
	return fmt.Sprintf("%s %s", neigh.IP, neigh.HardwareAddr)
}
