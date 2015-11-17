package daemon

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/opencontainers/runc/libcontainer/label"
)

func parseSecurityOpt(container *Container, config *runconfig.HostConfig) error {
	var (
		labelOpts []string
		err       error
	)

	for _, opt := range config.SecurityOpt {
		con := strings.SplitN(opt, ":", 2)
		if len(con) == 1 {
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
		switch con[0] {
		case "label":
			labelOpts = append(labelOpts, con[1])
		default:
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
	}

	container.ProcessLabel, container.MountLabel, err = label.InitLabels(labelOpts)
	return err
}

func checkKernel() error {
	// Check for unsupported kernel versions
	// FIXME: it would be cleaner to not test for specific versions, but rather
	// test for specific functionalities.
	// Unfortunately we can't test for the feature "does not cause a kernel panic"
	// without actually causing a kernel panic, so we need this workaround until
	// the circumstances of pre-3.10 crashes are clearer.
	// For details see https://github.com/docker/docker/issues/407
	if k, err := kernel.GetKernelVersion(); err != nil {
		logrus.Warnf("%s", err)
	} else {
		if kernel.CompareKernelVersion(k, &kernel.KernelVersionInfo{Kernel: 9, Major: 3}) < 0 {
			if os.Getenv("DOCKER_NOWARN_KERNEL_VERSION") == "" {
				logrus.Warnf("You are running BSD kernel version %s, which might be unstable running docker. Please upgrade your kernel to 9.3.", k.String())
			}
		}
	}
	return nil
}

// checkConfigOptions checks for mutually incompatible config options
func checkConfigOptions(config *Config) error {
	// Check for mutually incompatible config options
	if config.Bridge.Iface != "" && config.Bridge.IP != "" {
		return fmt.Errorf("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}
	return nil
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *Config, driverName string) error {
	return nil
}

func initBridgeDriver(controller libnetwork.NetworkController, config *Config) error {
	option := options.Generic{
		"EnableIPForwarding": config.Bridge.EnableIPForward}

	if err := controller.ConfigureNetworkDriver("bridge", options.Generic{netlabel.GenericData: option}); err != nil {
		return fmt.Errorf("Error initializing bridge driver: %v", err)
	}

	netOption := options.Generic{
		"BridgeName":          config.Bridge.Iface,
		"Mtu":                 config.Mtu,
		"EnableIPMasquerade":  config.Bridge.EnableIPMasq,
		"EnableICC":           config.Bridge.InterContainerCommunication,
		"EnableUserlandProxy": config.Bridge.EnableUserlandProxy,
	}

	if config.Bridge.IP != "" {
		ip, bipNet, err := net.ParseCIDR(config.Bridge.IP)
		if err != nil {
			return err
		}

		bipNet.IP = ip
		netOption["AddressIPv4"] = bipNet
	}

	if config.Bridge.FixedCIDR != "" {
		_, fCIDR, err := net.ParseCIDR(config.Bridge.FixedCIDR)
		if err != nil {
			return err
		}

		netOption["FixedCIDR"] = fCIDR
	}

	if config.Bridge.FixedCIDRv6 != "" {
		_, fCIDRv6, err := net.ParseCIDR(config.Bridge.FixedCIDRv6)
		if err != nil {
			return err
		}

		netOption["FixedCIDRv6"] = fCIDRv6
	}

	if config.Bridge.DefaultGatewayIPv4 != nil {
		netOption["DefaultGatewayIPv4"] = config.Bridge.DefaultGatewayIPv4
	}

	if config.Bridge.DefaultGatewayIPv6 != nil {
		netOption["DefaultGatewayIPv6"] = config.Bridge.DefaultGatewayIPv6
	}

	// --ip processing
	if config.Bridge.DefaultIP != nil {
		netOption["DefaultBindingIP"] = config.Bridge.DefaultIP
	}

	// Initialize default network on "bridge" with the same name
	_, err := controller.NewNetwork("bridge", "bridge",
		libnetwork.NetworkOptionGeneric(options.Generic{
			netlabel.GenericData: netOption,
			netlabel.EnableIPv6:  config.Bridge.EnableIPv6,
		}))
	if err != nil {
		return fmt.Errorf("Error creating default \"bridge\" network: %v", err)
	}
	return nil
}
