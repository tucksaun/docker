package libnetwork

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
)

func initDrivers(dc driverapi.DriverCallback) error {
	for _, fn := range [](func(driverapi.DriverCallback) error){
		bridge.Init,
	} {
		if err := fn(dc); err != nil {
			return err
		}
	}
	return nil
}
