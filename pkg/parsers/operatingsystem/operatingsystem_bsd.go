// +build freebsd darwin

package operatingsystem

import (
	"os"
	"syscall"
)

func GetOperatingSystem() (string, error) {
	val, err := syscall.Sysctl("kern.ostype")
	if err != nil {
		return "", err
	}

	return val, nil
}

func IsContainerized() (bool, error) {
	if _, err := os.Stat("/.dockerinit"); err != nil && os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}
