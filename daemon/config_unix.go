package daemon

import (
	"net"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
)

var (
	defaultPidFile = "/var/run/docker.pid"
	defaultGraph   = "/var/lib/docker"
)

// bridgeConfig stores all the bridge driver specific
// configuration.
type unixBridgeConfig struct {
	EnableIPv6                  bool
	EnableIPForward             bool
	EnableIPMasq                bool
	EnableUserlandProxy         bool
	DefaultIP                   net.IP
	Iface                       string
	IP                          string
	FixedCIDR                   string
	FixedCIDRv6                 string
	DefaultGatewayIPv4          net.IP
	DefaultGatewayIPv6          net.IP
	InterContainerCommunication bool
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallCommonUnixFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags(cmd, usageFn)

	// Then platform-specific install flags
	cmd.StringVar(&config.SocketGroup, []string{"G", "-group"}, "docker", usageFn("Group for the unix socket"))
	config.Ulimits = make(map[string]*ulimit.Ulimit)
	cmd.Var(opts.NewUlimitOpt(&config.Ulimits), []string{"-default-ulimit"}, usageFn("Set default ulimits for containers"))
	cmd.BoolVar(&config.Bridge.EnableIPMasq, []string{"-ip-masq"}, true, usageFn("Enable IP masquerading"))
	cmd.BoolVar(&config.Bridge.EnableIPv6, []string{"-ipv6"}, false, usageFn("Enable IPv6 networking"))
	cmd.StringVar(&config.Bridge.IP, []string{"#bip", "-bip"}, "", usageFn("Specify network bridge IP"))
	cmd.StringVar(&config.Bridge.Iface, []string{"b", "-bridge"}, "", usageFn("Attach containers to a network bridge"))
	cmd.StringVar(&config.Bridge.FixedCIDR, []string{"-fixed-cidr"}, "", usageFn("IPv4 subnet for fixed IPs"))
	cmd.StringVar(&config.Bridge.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", usageFn("IPv6 subnet for fixed IPs"))
	cmd.Var(opts.NewIpOpt(&config.Bridge.DefaultGatewayIPv4, ""), []string{"-default-gateway"}, usageFn("Container default gateway IPv4 address"))
	cmd.Var(opts.NewIpOpt(&config.Bridge.DefaultGatewayIPv6, ""), []string{"-default-gateway-v6"}, usageFn("Container default gateway IPv6 address"))
	cmd.BoolVar(&config.Bridge.InterContainerCommunication, []string{"#icc", "-icc"}, true, usageFn("Enable inter-container communication"))
	cmd.Var(opts.NewIpOpt(&config.Bridge.DefaultIP, "0.0.0.0"), []string{"#ip", "-ip"}, usageFn("Default IP when binding container ports"))
	cmd.BoolVar(&config.Bridge.EnableUserlandProxy, []string{"-userland-proxy"}, true, usageFn("Use userland proxy for loopback traffic"))
}
