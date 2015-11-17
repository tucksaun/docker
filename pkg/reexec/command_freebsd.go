// +build freebsd

package reexec

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Self returns the path to the current processes binary
func Self() string {
	name := os.Args[0]
	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err == nil {
			return lp
		}
	}
	// handle conversion of relative paths to absolute
	if absName, err := filepath.Abs(name); err == nil {
		return absName
	}
	// if we coudn't get absolute name, return original
	// (NOTE: Go only errors on Abs() if os.Getwd fails)
	return name
}

func Command(args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: Self(),
		Args: args,
		//SysProcAttr: &syscall.SysProcAttr{
		//	Pdeathsig: syscall.SIGTERM,
		//},
	}
}
