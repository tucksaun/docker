package bridge

import (
	"net"
	"sync"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipallocator"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/types"
)

const (
	networkType             = "bridge"
	maxAllocatePortAttempts = 10
)

var (
	ipAllocator *ipallocator.IPAllocator
)

// configuration info for the "bridge" driver.
type configuration struct {
	EnableIPForwarding bool
}

// networkConfiguration for network specific configuration
type networkConfiguration struct {
	BridgeName            string
	AddressIPv4           *net.IPNet
	FixedCIDR             *net.IPNet
	FixedCIDRv6           *net.IPNet
	EnableIPv6            bool
	EnableIPTables        bool
	EnableIPMasquerade    bool
	EnableICC             bool
	Mtu                   int
	DefaultGatewayIPv4    net.IP
	DefaultGatewayIPv6    net.IP
	DefaultBindingIP      net.IP
	AllowNonDefaultBridge bool
	EnableUserlandProxy   bool
}

// endpointConfiguration represents the user specified configuration for the sandbox endpoint
type endpointConfiguration struct {
	MacAddress   net.HardwareAddr
	PortBindings []types.PortBinding
	ExposedPorts []types.TransportPort
}

// containerConfiguration represents the user specified configuration for a container
type containerConfiguration struct {
	ParentEndpoints []string
	ChildEndpoints  []string
}

type bridgeEndpoint struct {
	id              types.UUID
	srcName         string
	addr            *net.IPNet
	addrv6          *net.IPNet
	macAddress      net.HardwareAddr
	config          *endpointConfiguration // User specified parameters
	containerConfig *containerConfiguration
	portMapping     []types.PortBinding // Operation port bindings
}

type bridgeNetwork struct {
	id         types.UUID
	bridge     *bridgeInterface // The bridge's L3 interface
	config     *networkConfiguration
	endpoints  map[types.UUID]*bridgeEndpoint // key: endpoint id
	portMapper *portmapper.PortMapper
	sync.Mutex
}

type driver struct {
	config   *configuration
	network  *bridgeNetwork
	networks map[types.UUID]*bridgeNetwork
	sync.Mutex
}

func init() {
	ipAllocator = ipallocator.New()
}

// New constructs a new bridge driver
func newDriver() driverapi.Driver {
	return &driver{networks: map[types.UUID]*bridgeNetwork{}}
}

func (d *driver) Type() string {
	return networkType
}

func parseEndpointOptions(epOptions map[string]interface{}) (*endpointConfiguration, error) {
	if epOptions == nil {
		return nil, nil
	}

	ec := &endpointConfiguration{}

	if opt, ok := epOptions[netlabel.MacAddress]; ok {
		if mac, ok := opt.(net.HardwareAddr); ok {
			ec.MacAddress = mac
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.PortMap]; ok {
		if bs, ok := opt.([]types.PortBinding); ok {
			ec.PortBindings = bs
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.ExposedPorts]; ok {
		if ports, ok := opt.([]types.TransportPort); ok {
			ec.ExposedPorts = ports
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	return ec, nil
}

func parseContainerOptions(cOptions map[string]interface{}) (*containerConfiguration, error) {
	if cOptions == nil {
		return nil, nil
	}
	genericData := cOptions[netlabel.GenericData]
	if genericData == nil {
		return nil, nil
	}
	switch opt := genericData.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &containerConfiguration{})
		if err != nil {
			return nil, err
		}
		return opaqueConfig.(*containerConfiguration), nil
	case *containerConfiguration:
		return opt, nil
	default:
		return nil, nil
	}
}

// Generate a IEEE802 compliant MAC address from the given IP address.
//
// The generator is guaranteed to be consistent: the same IP will always yield the same
// MAC address. This is to avoid ARP cache issues.
func generateMacAddr(ip net.IP) net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)

	// The first byte of the MAC address has to comply with these rules:
	// 1. Unicast: Set the least-significant bit to 0.
	// 2. Address is locally administered: Set the second-least-significant bit (U/L) to 1.
	// 3. As "small" as possible: The veth address has to be "smaller" than the bridge address.
	hw[0] = 0x02

	// The first 24 bits of the MAC represent the Organizationally Unique Identifier (OUI).
	// Since this address is locally administered, we can do whatever we want as long as
	// it doesn't conflict with other addresses.
	hw[1] = 0x42

	// Insert the IP address into the last 32 bits of the MAC address.
	// This is a simple way to guarantee the address will be consistent and unique.
	copy(hw[2:], ip.To4())

	return hw
}

func electMacAddress(epConfig *endpointConfiguration, ip net.IP) net.HardwareAddr {
	if epConfig != nil && epConfig.MacAddress != nil {
		return epConfig.MacAddress
	}
	return generateMacAddr(ip)
}
