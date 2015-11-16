// +build freebsd darwin

package kernel

import (
	"syscall"
)

/*
#include <sys/utsname.h>
*/
import "C"

type Utsname struct {
	Sysname, Nodename, Release, Version, Machine string
}

func uname() (*Utsname, error) {
	var utsname C.struct_utsname

	if err := C.uname(&utsname); err != 0 {
		return nil, syscall.Errno(err)
	}

	return &Utsname{
		Sysname: C.GoString(&utsname.sysname[0]),
		Nodename: C.GoString(&utsname.nodename[0]),
		Release: C.GoString(&utsname.release[0]),
		Version: C.GoString(&utsname.version[0]),
		Machine: C.GoString(&utsname.machine[0]),
	}, nil
}
