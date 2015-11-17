package daemon

import (
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
)

var (
	defaultExec    = "jails"
)

// Config defines the configuration of a docker daemon.
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
type Config struct {
	CommonConfig

	// Fields below here are platform specific.
	SocketGroup          string
	Ulimits              map[string]*ulimit.Ulimit
}

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig unixBridgeConfig


// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonUnixFlags(cmd, usageFn)

	// Then platform-specific install flags
	cmd.BoolVar(&config.Bridge.EnableIPForward, []string{"#ip-forward", "-ip-forward"}, true, usageFn("Enable net.inet.ip.forwarding"))

	config.attachExperimentalFlags(cmd, usageFn)
}
