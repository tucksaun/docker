package bridge

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

type bridgeNetwork struct {
	id         types.UUID
	bridge     *bridgeInterface // The bridge's L3 interface
	config     *networkConfiguration
	endpoints  map[types.UUID]*bridgeEndpoint // key: endpoint id
	portMapper *portmapper.PortMapper
	sync.Mutex
}

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

	setupNetworkIsolationRules := func(config *networkConfiguration, i *bridgeInterface) error {
		if err := network.isolateNetwork(networkList, true); err != nil {
			if err := network.isolateNetwork(networkList, false); err != nil {
				logrus.Warnf("Failed on removing the inter-network iptables rules on cleanup: %v", err)
			}
			return err
		}
		return nil
	}

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

		//We want to track firewalld configuration so that
		//if it is started/reloaded, the rules can be applied correctly
		{config.EnableIPTables, network.setupFirewalld},

		// Setup DefaultGatewayIPv4
		{config.DefaultGatewayIPv4 != nil, setupGatewayIPv4},

		// Setup DefaultGatewayIPv6
		{config.DefaultGatewayIPv6 != nil, setupGatewayIPv6},

		// Add inter-network communication rules.
		{config.EnableIPTables, setupNetworkIsolationRules},

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

func addToBridge(ifaceName, bridgeName string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("could not find interface %s: %v", ifaceName, err)
	}
	if err = netlink.LinkSetMaster(link,
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: bridgeName}}); err != nil {
		logrus.Debugf("Failed to add %s to bridge via netlink.Trying ioctl: %v", ifaceName, err)
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return fmt.Errorf("could not find network interface %s: %v", ifaceName, err)
		}

		master, err := net.InterfaceByName(bridgeName)
		if err != nil {
			return fmt.Errorf("could not find bridge %s: %v", bridgeName, err)
		}

		return ioctlAddToBridge(iface, master)
	}
	return nil
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	var (
		ipv6Addr *net.IPNet
		err      error
	)

	defer sandbox.InitOSContext()()

	if epInfo == nil {
		return errors.New("invalid endpoint info passed")
	}

	if len(epInfo.Interfaces()) != 0 {
		return errors.New("non empty interface list passed to bridge(local) driver")
	}

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}

	// Endpoint with that id exists either on desired or other sandbox
	if ep != nil {
		return driverapi.ErrEndpointExists(eid)
	}

	// Try to convert the options to endpoint configuration
	epConfig, err := parseEndpointOptions(epOptions)
	if err != nil {
		return err
	}

	// Create and add the endpoint
	n.Lock()
	endpoint := &bridgeEndpoint{id: eid, config: epConfig}
	n.endpoints[eid] = endpoint
	n.Unlock()

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, eid)
			n.Unlock()
		}
	}()

	// Generate a name for what will be the host side pipe interface
	hostIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate a name for what will be the sandbox side pipe interface
	containerIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfName, TxQLen: 0},
		PeerName:  containerIfName}
	if err = netlink.LinkAdd(veth); err != nil {
		return err
	}

	// Get the host side pipe interface handler
	host, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	// Get the sandbox side pipe interface handler
	sbox, err := netlink.LinkByName(containerIfName)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(sbox)
		}
	}()

	n.Lock()
	config := n.config
	n.Unlock()

	// Add bridge inherited attributes to pipe interfaces
	if config.Mtu != 0 {
		err = netlink.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return err
		}
		err = netlink.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return err
		}
	}

	// Attach host side pipe interface into the bridge
	if err = addToBridge(hostIfName, config.BridgeName); err != nil {
		return fmt.Errorf("adding interface %s to bridge %s failed: %v", hostIfName, config.BridgeName, err)
	}

//	if !config.EnableUserlandProxy {
//		err = setHairpinMode(host, true)
//		if err != nil {
//			return err
//		}
//	}

	// v4 address for the sandbox side pipe interface
	ip4, err := ipAllocator.RequestIP(n.bridge.bridgeIPv4, nil)
	if err != nil {
		return err
	}
	ipv4Addr := &net.IPNet{IP: ip4, Mask: n.bridge.bridgeIPv4.Mask}

	// Down the interface before configuring mac address.
	if err = netlink.LinkSetDown(sbox); err != nil {
		return fmt.Errorf("could not set link down for container interface %s: %v", containerIfName, err)
	}

	// Set the sbox's MAC. If specified, use the one configured by user, otherwise generate one based on IP.
	mac := electMacAddress(epConfig, ip4)
	err = netlink.LinkSetHardwareAddr(sbox, mac)
	if err != nil {
		return fmt.Errorf("could not set mac address for container interface %s: %v", containerIfName, err)
	}
	endpoint.macAddress = mac

	// Up the host interface after finishing all netlink configuration
	if err = netlink.LinkSetUp(host); err != nil {
		return fmt.Errorf("could not set link up for host interface %s: %v", hostIfName, err)
	}

	// v6 address for the sandbox side pipe interface
	ipv6Addr = &net.IPNet{}
	if config.EnableIPv6 {
		var ip6 net.IP

		network := n.bridge.bridgeIPv6
		if config.FixedCIDRv6 != nil {
			network = config.FixedCIDRv6
		}

		ones, _ := network.Mask.Size()
		if ones <= 80 {
			ip6 = make(net.IP, len(network.IP))
			copy(ip6, network.IP)
			for i, h := range mac {
				ip6[i+10] = h
			}
		}

		ip6, err := ipAllocator.RequestIP(network, ip6)
		if err != nil {
			return err
		}

		ipv6Addr = &net.IPNet{IP: ip6, Mask: network.Mask}
	}

	// Create the sandbox side pipe interface
	endpoint.srcName = containerIfName
	endpoint.addr = ipv4Addr

	if config.EnableIPv6 {
		endpoint.addrv6 = ipv6Addr
	}

	err = epInfo.AddInterface(ifaceID, endpoint.macAddress, *ipv4Addr, *ipv6Addr)
	if err != nil {
		return err
	}

	// Program any required port mapping and store them in the endpoint
	endpoint.portMapping, err = n.allocatePorts(epConfig, endpoint, config.DefaultBindingIP, config.EnableUserlandProxy)
	if err != nil {
		return err
	}

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	var err error

	defer sandbox.InitOSContext()()

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity Check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check endpoint id and if an endpoint is actually there
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}
	if ep == nil {
		return EndpointNotFoundError(eid)
	}

	// Remove it
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			n.Lock()
			if _, ok := n.endpoints[eid]; !ok {
				n.endpoints[eid] = ep
			}
			n.Unlock()
		}
	}()

	// Remove port mappings. Do not stop endpoint delete on unmap failure
	n.releasePorts(ep)

	// Release the v4 address allocated to this endpoint's sandbox interface
	err = ipAllocator.ReleaseIP(n.bridge.bridgeIPv4, ep.addr.IP)
	if err != nil {
		return err
	}

	n.Lock()
	config := n.config
	n.Unlock()

	// Release the v6 address allocated to this endpoint's sandbox interface
	if config.EnableIPv6 {
		network := n.bridge.bridgeIPv6
		if config.FixedCIDRv6 != nil {
			network = config.FixedCIDRv6
		}
		err := ipAllocator.ReleaseIP(network, ep.addrv6.IP)
		if err != nil {
			return err
		}
	}

	// Try removal of link. Discard error: link pair might have
	// already been deleted by sandbox delete.
	link, err := netlink.LinkByName(ep.srcName)
	if err == nil {
		netlink.LinkDel(link)
	}

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