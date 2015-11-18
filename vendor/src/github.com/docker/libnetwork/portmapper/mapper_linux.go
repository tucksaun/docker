package portmapper

import (
	"net"

	"github.com/docker/libnetwork/iptables"
)

// PortMapper manages the network address translation
type portMapper struct {
	chain *iptables.Chain
}

// SetIptablesChain sets the specified chain into portmapper
func (pm *portMapper) SetIptablesChain(c *iptables.Chain) {
	pm.chain = c
}

func (pm *portMapper) forward(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.chain == nil {
		return nil
	}
	return pm.chain.Forward(iptables.Append, sourceIP, sourcePort, proto, containerIP, containerPort)
}

func (pm *portMapper) cleanForward(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.chain == nil {
		return nil
	}
	return pm.chain.Forward(iptables.Delete, sourceIP, sourcePort, proto, containerIP, containerPort)
}
