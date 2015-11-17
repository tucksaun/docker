// +build !linux

package lxc

type InitArgs struct {}

func finalizeNamespace(args *InitArgs) error {
	panic("Not supported on this platform")
}
