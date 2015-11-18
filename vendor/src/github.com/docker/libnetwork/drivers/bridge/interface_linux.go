package bridge

import "github.com/vishvananda/netlink"

const (
	// DefaultBridgeName is the default name for the bridge interface managed
	// by the driver when unspecified by the caller.
	DefaultBridgeName = "docker0"
)

// newInterface creates a new bridge interface structure. It attempts to find
// an already existing device identified by the configuration BridgeName field,
// or the default bridge name when unspecified), but doesn't attempt to create
// one when missing
func newInterface(config *networkConfiguration) *bridgeInterface {
	i := &bridgeInterface{}

	// Initialize the bridge name to the default if unspecified.
	if config.BridgeName == "" {
		config.BridgeName = DefaultBridgeName
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, _ = netlink.LinkByName(config.BridgeName)
	return i
}