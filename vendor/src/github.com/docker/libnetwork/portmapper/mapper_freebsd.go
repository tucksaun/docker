package portmapper

import (
	"net"
)

// PortMapper manages the network address translation
type portMapper struct {}

func (pm *portMapper) forward(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	// TODO FreeBSD: Implement PortForwarding.
	return nil
}

func (pm *portMapper) cleanForward(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	// TODO FreeBSD: Implement PortForwarding.
	return nil
}
