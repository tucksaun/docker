package bridge

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

const (
	vethPrefix          = "veth"
	vethLen             = 7
	containerVethPrefix = "eth"
	ifaceID             = 1
)

// Init registers a new instance of bridge driver
func Init(dc driverapi.DriverCallback) error {
	// try to modprobe bridge first
	// see gh#12177
	if out, err := exec.Command("kldload", "-n", "pf").Output(); err != nil {
		logrus.Warnf("Running kldload pf failed with message: %s, error: %v", out, err)
	}

	c := driverapi.Capability{
		Scope: driverapi.LocalScope,
	}
	return dc.RegisterDriver(networkType, newDriver(), c)
}

// fromMap retrieve the configuration data from the map form.
func (c *networkConfiguration) fromMap(data map[string]interface{}) error {
	var err error

	if i, ok := data["BridgeName"]; ok && i != nil {
		if c.BridgeName, ok = i.(string); !ok {
			return types.BadRequestErrorf("invalid type for BridgeName value")
		}
	}

	if i, ok := data["Mtu"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.Mtu, err = strconv.Atoi(s); err != nil {
				return types.BadRequestErrorf("failed to parse Mtu value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for Mtu value")
		}
	}

	if i, ok := data["EnableIPv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPv6, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPv6 value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPv6 value")
		}
	}

	if i, ok := data["EnableIPTables"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPTables, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPTables value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPTables value")
		}
	}

	if i, ok := data["EnableIPMasquerade"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPMasquerade, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPMasquerade value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPMasquerade value")
		}
	}

	if i, ok := data["EnableICC"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableICC, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableICC value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableICC value")
		}
	}

	if i, ok := data["AllowNonDefaultBridge"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.AllowNonDefaultBridge, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse AllowNonDefaultBridge value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for AllowNonDefaultBridge value")
		}
	}

	if i, ok := data["AddressIPv4"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.AddressIPv4 = nw
			} else {
				return types.BadRequestErrorf("failed to parse AddressIPv4 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for AddressIPv4 value")
		}
	}

	if i, ok := data["FixedCIDR"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.FixedCIDR = nw
			} else {
				return types.BadRequestErrorf("failed to parse FixedCIDR value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for FixedCIDR value")
		}
	}

	if i, ok := data["FixedCIDRv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.FixedCIDRv6 = nw
			} else {
				return types.BadRequestErrorf("failed to parse FixedCIDRv6 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for FixedCIDRv6 value")
		}
	}

	if i, ok := data["DefaultGatewayIPv4"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultGatewayIPv4 = net.ParseIP(s); c.DefaultGatewayIPv4 == nil {
				return types.BadRequestErrorf("failed to parse DefaultGatewayIPv4 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultGatewayIPv4 value")
		}
	}

	if i, ok := data["DefaultGatewayIPv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultGatewayIPv6 = net.ParseIP(s); c.DefaultGatewayIPv6 == nil {
				return types.BadRequestErrorf("failed to parse DefaultGatewayIPv6 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultGatewayIPv6 value")
		}
	}

	if i, ok := data["DefaultBindingIP"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultBindingIP = net.ParseIP(s); c.DefaultBindingIP == nil {
				return types.BadRequestErrorf("failed to parse DefaultBindingIP value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultBindingIP value")
		}
	}
	return nil
}

func (n *bridgeNetwork) getEndpoint(eid types.UUID) (*bridgeEndpoint, error) {
	n.Lock()
	defer n.Unlock()

	if eid == "" {
		return nil, InvalidEndpointIDError(eid)
	}

	if ep, ok := n.endpoints[eid]; ok {
		return ep, nil
	}

	return nil, nil
}

// Checks whether this network's configuration for the network with this id conflicts with any of the passed networks
func (c *networkConfiguration) conflictsWithNetworks(id types.UUID, others []*bridgeNetwork) error {
	for _, nw := range others {

		nw.Lock()
		nwID := nw.id
		nwConfig := nw.config
		nwBridge := nw.bridge
		nw.Unlock()

		if nwID == id {
			continue
		}
		// Verify the name (which may have been set by newInterface()) does not conflict with
		// existing bridge interfaces. Ironically the system chosen name gets stored in the config...
		// Basically we are checking if the two original configs were both empty.
		if nwConfig.BridgeName == c.BridgeName {
			return types.ForbiddenErrorf("conflicts with network %s (%s) by bridge name", nwID, nwConfig.BridgeName)
		}
		// If this network config specifies the AddressIPv4, we need
		// to make sure it does not conflict with any previously allocated
		// bridges. This could not be completely caught by the config conflict
		// check, because networks which config does not specify the AddressIPv4
		// get their address and subnet selected by the driver (see electBridgeIPv4())
		if c.AddressIPv4 != nil {
			if nwBridge.bridgeIPv4.Contains(c.AddressIPv4.IP) ||
				c.AddressIPv4.Contains(nwBridge.bridgeIPv4.IP) {
				return types.ForbiddenErrorf("conflicts with network %s (%s) by ip network", nwID, nwConfig.BridgeName)
			}
		}
	}

	return nil
}

func (d *driver) Config(option map[string]interface{}) error {
	var config *configuration

	d.Lock()
	defer d.Unlock()

	if d.config != nil {
		return &ErrConfigExists{}
	}

	genericData, ok := option[netlabel.GenericData]
	if ok && genericData != nil {
		switch opt := genericData.(type) {
		case options.Generic:
			opaqueConfig, err := options.GenerateFromModel(opt, &configuration{})
			if err != nil {
				return err
			}
			config = opaqueConfig.(*configuration)
		case *configuration:
			config = opt
		default:
			return &ErrInvalidDriverConfig{}
		}

		d.config = config
	} else {
		config = &configuration{}
	}

	/*if config.EnableIPForwarding {
		return setupIPForwarding(config)
	}*/

	return nil
}

func (d *driver) getNetwork(id types.UUID) (*bridgeNetwork, error) {
	d.Lock()
	defer d.Unlock()

	if id == "" {
		return nil, types.BadRequestErrorf("invalid network id: %s", id)
	}

	if nw, ok := d.networks[id]; ok {
		return nw, nil
	}

	return nil, types.NotFoundErrorf("network not found: %s", id)
}

func parseNetworkGenericOptions(data interface{}) (*networkConfiguration, error) {
	var (
		err    error
		config *networkConfiguration
	)

	switch opt := data.(type) {
	case *networkConfiguration:
		config = opt
	case map[string]interface{}:
		config = &networkConfiguration{
			EnableICC:          true,
			EnableIPTables:     true,
			EnableIPMasquerade: true,
		}
		err = config.fromMap(opt)
	case options.Generic:
		var opaqueConfig interface{}
		if opaqueConfig, err = options.GenerateFromModel(opt, config); err == nil {
			config = opaqueConfig.(*networkConfiguration)
		}
	default:
		err = types.BadRequestErrorf("do not recognize network configuration format: %T", opt)
	}

	return config, err
}

func parseNetworkOptions(option options.Generic) (*networkConfiguration, error) {
	var err error
	config := &networkConfiguration{}

	// Parse generic label first, config will be re-assigned
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		if config, err = parseNetworkGenericOptions(genData); err != nil {
			return nil, err
		}
	}

	// Process well-known labels next
	if _, ok := option[netlabel.EnableIPv6]; ok {
		config.EnableIPv6 = option[netlabel.EnableIPv6].(bool)
	}

	// Finally validate the configuration
	if err = config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Returns the non link-local IPv6 subnet for the containers attached to this bridge if found, nil otherwise
func getV6Network(config *networkConfiguration, i *bridgeInterface) *net.IPNet {
	if config.FixedCIDRv6 != nil {
		return config.FixedCIDRv6
	}

	if i.bridgeIPv6 != nil && i.bridgeIPv6.IP != nil && !i.bridgeIPv6.IP.IsLinkLocalUnicast() {
		return i.bridgeIPv6
	}

	return nil
}

// Return a slice of networks over which caller can iterate safely
func (d *driver) getNetworks() []*bridgeNetwork {
	d.Lock()
	defer d.Unlock()

	ls := make([]*bridgeNetwork, 0, len(d.networks))
	for _, nw := range d.networks {
		ls = append(ls, nw)
	}
	return ls
}

// Create a new network using bridge plugin
func (d *driver) CreateNetwork(id types.UUID, option map[string]interface{}) error {
	var err error

	defer sandbox.InitOSContext()()

	// Sanity checks
	d.Lock()
	if _, ok := d.networks[id]; ok {
		d.Unlock()
		return types.ForbiddenErrorf("network %s exists", id)
	}
	d.Unlock()

	// Parse and validate the config. It should not conflict with existing networks' config
	config, err := parseNetworkOptions(option)
	if err != nil {
		return err
	}
	networkList := d.getNetworks()
	for _, nw := range networkList {
		nw.Lock()
		nwConfig := nw.config
		nw.Unlock()
		if nwConfig.Conflicts(config) {
			return types.ForbiddenErrorf("conflicts with network %s (%s)", nw.id, nw.config.BridgeName)
		}
	}

	// Create and set network handler in driver
	network := &bridgeNetwork{
		id:         id,
		endpoints:  make(map[types.UUID]*bridgeEndpoint),
		config:     config,
		portMapper: portmapper.New(),
	}

	d.Lock()
	d.networks[id] = network
	d.Unlock()

	// On failure make sure to reset driver network handler to nil
	defer func() {
		if err != nil {
			d.Lock()
			delete(d.networks, id)
			d.Unlock()
		}
	}()

	// Create or retrieve the bridge L3 interface
	bridgeIface := newInterface(config)
	network.bridge = bridgeIface

	// Verify the network configuration does not conflict with previously installed
	// networks. This step is needed now because driver might have now set the bridge
	// name on this config struct. And because we need to check for possible address
	// conflicts, so we need to check against operationa lnetworks.
	if err = config.conflictsWithNetworks(id, networkList); err != nil {
		return err
	}

	/*setupNetworkIsolationRules := func(config *networkConfiguration, i *bridgeInterface) error {
		if err := network.isolateNetwork(networkList, true); err != nil {
			if err := network.isolateNetwork(networkList, false); err != nil {
				logrus.Warnf("Failed on removing the inter-network iptables rules on cleanup: %v", err)
			}
			return err
		}
		return nil
	}*/

	// Prepare the bridge setup configuration
	bridgeSetup := newBridgeSetup(config, bridgeIface)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIface.exists()
	if !bridgeAlreadyExists {
		bridgeSetup.queueStep(setupDevice)
	}

	// Even if a bridge exists try to setup IPv4.
	bridgeSetup.queueStep(setupBridgeIPv4)

	enableIPv6Forwarding := false
	if d.config != nil && d.config.EnableIPForwarding && config.FixedCIDRv6 != nil {
		enableIPv6Forwarding = true
	}

	// Conditionally queue setup steps depending on configuration values.
	for _, step := range []struct {
		Condition bool
		Fn        setupStep
	}{
		// Enable IPv6 on the bridge if required. We do this even for a
		// previously  existing bridge, as it may be here from a previous
		// installation where IPv6 wasn't supported yet and needs to be
		// assigned an IPv6 link-local address.
		{config.EnableIPv6, setupBridgeIPv6},

		// We ensure that the bridge has the expectedIPv4 and IPv6 addresses in
		// the case of a previously existing device.
		{bridgeAlreadyExists, setupVerifyAndReconcile},

		// Setup the bridge to allocate containers IPv4 addresses in the
		// specified subnet.
		{config.FixedCIDR != nil, setupFixedCIDRv4},

		// Setup the bridge to allocate containers global IPv6 addresses in the
		// specified subnet.
		{config.FixedCIDRv6 != nil, setupFixedCIDRv6},

		// Enable IPv6 Forwarding
		{enableIPv6Forwarding, setupIPv6Forwarding},

		// Setup Loopback Adresses Routing
		{!config.EnableUserlandProxy, setupLoopbackAdressesRouting},

		// Setup IPTables.
		//{config.EnableIPTables, network.setupIPTables},

		//We want to track firewalld configuration so that
		//if it is started/reloaded, the rules can be applied correctly
		//{config.EnableIPTables, network.setupFirewalld},

		// Setup DefaultGatewayIPv4
		{config.DefaultGatewayIPv4 != nil, setupGatewayIPv4},

		// Setup DefaultGatewayIPv6
		{config.DefaultGatewayIPv6 != nil, setupGatewayIPv6},

		// Add inter-network communication rules.
		//{config.EnableIPTables, setupNetworkIsolationRules},

		//Configure bridge networking filtering if ICC is off and IP tables are enabled
		{!config.EnableICC && config.EnableIPTables, setupBridgeNetFiltering},
	} {
		if step.Condition {
			bridgeSetup.queueStep(step.Fn)
		}
	}

	// Block bridge IP from being allocated.
	bridgeSetup.queueStep(allocateBridgeIP)
	// Apply the prepared list of steps, and abort at the first error.
	bridgeSetup.queueStep(setupDeviceUp)
	if err = bridgeSetup.apply(); err != nil {
		return err
	}

	return nil
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	var err error

	defer sandbox.InitOSContext()()

	// Get network handler and remove it from driver
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", nid)
	}

	n.Lock()
	config := n.config
	n.Unlock()

	if config.BridgeName == DefaultBridgeName {
		return types.ForbiddenErrorf("default network of type \"%s\" cannot be deleted", networkType)
	}

	d.Lock()
	delete(d.networks, nid)
	d.Unlock()

	// On failure set network handler back in driver, but
	// only if is not already taken over by some other thread
	defer func() {
		if err != nil {
			d.Lock()
			if _, ok := d.networks[nid]; !ok {
				d.networks[nid] = n
			}
			d.Unlock()
		}
	}()

	// Sanity check
	if n == nil {
		err = driverapi.ErrNoNetwork(nid)
		return err
	}

	// Cannot remove network if endpoints are still present
	if len(n.endpoints) != 0 {
		err = ActiveEndpointsError(n.id)
		return err
	}

	// In case of failures after this point, restore the network isolation rules
	/*nwList := d.getNetworks()
	defer func() {
		if err != nil {
			if err := n.isolateNetwork(nwList, true); err != nil {
				logrus.Warnf("Failed on restoring the inter-network iptables rules on cleanup: %v", err)
			}
		}
	}()

	// Remove inter-network communication rules.
	err = n.isolateNetwork(nwList, false)
	if err != nil {
		return err
	}*/

	// Programming
	//err = netlink.LinkDel(n.bridge.Link)

	return err
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	// TODO FreeBSD
	return nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	// TODO FreeBSD
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()
	if !ok {
		return nil, types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return nil, driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, driverapi.ErrNoEndpoint(eid)
	}

	m := make(map[string]interface{})

	if ep.config.ExposedPorts != nil {
		// Return a copy of the config data
		epc := make([]types.TransportPort, 0, len(ep.config.ExposedPorts))
		for _, tp := range ep.config.ExposedPorts {
			epc = append(epc, tp.GetCopy())
		}
		m[netlabel.ExposedPorts] = epc
	}

	if ep.portMapping != nil {
		// Return a copy of the operational data
		pmc := make([]types.PortBinding, 0, len(ep.portMapping))
		for _, pm := range ep.portMapping {
			pmc = append(pmc, pm.GetCopy())
		}
		m[netlabel.PortMap] = pmc
	}

	if len(ep.macAddress) != 0 {
		m[netlabel.MacAddress] = ep.macAddress
	}

	return m, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid types.UUID, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	defer sandbox.InitOSContext()()

	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	for _, iNames := range jinfo.InterfaceNames() {
		// Make sure to set names on the correct interface ID.
		if iNames.ID() == ifaceID {
			err = iNames.SetNames(endpoint.srcName, containerVethPrefix)
			if err != nil {
				return err
			}
		}
	}

	err = jinfo.SetGateway(network.bridge.gatewayIPv4)
	if err != nil {
		return err
	}

	err = jinfo.SetGatewayIPv6(network.bridge.gatewayIPv6)
	if err != nil {
		return err
	}

	if !network.config.EnableICC {
		return d.link(network, endpoint, options, true)
	}

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid types.UUID) error {
	defer sandbox.InitOSContext()()

	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	if !network.config.EnableICC {
		return d.link(network, endpoint, nil, false)
	}

	return nil
}

func (d *driver) link(network *bridgeNetwork, endpoint *bridgeEndpoint, options map[string]interface{}, enable bool) error {
	// TODO FreeBSD
	return nil
}
