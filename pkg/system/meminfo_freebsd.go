package system

import (
	"syscall"
	"unsafe"
)

/*
#include <kvm.h>
*/
import "C"

// ReadMemInfo retrieves memory statistics of the host system and returns a
//  MemInfo type.
func ReadMemInfo() (*MemInfo, error) {
	meminfo := &MemInfo{}

	var (
		val      string
		pagesize uint64
		err      error
		buf      []byte
	)

	if val, err = syscall.Sysctl("hw.physmem"); err != nil {
		return nil, err
	}
	buf = []byte(val)
	meminfo.MemTotal = *(*uint64)(unsafe.Pointer(&buf[0]))

	if val, err = syscall.Sysctl("vm.swap_total"); err != nil {
		return nil, err
	}
	buf = []byte(val)
	meminfo.SwapTotal = *(*uint64)(unsafe.Pointer(&buf[0]))

	if val, err = syscall.Sysctl("hw.pagesize"); err != nil {
		return nil, err
	}
	buf = []byte(val)
	pagesize = *(*uint64)(unsafe.Pointer(&buf[0]))

	if val, err = syscall.Sysctl("vm.stats.vm.v_free_count"); err != nil {
		return nil, err
	}
	buf = []byte(val)
	meminfo.MemFree = *(*uint64)(unsafe.Pointer(&buf[0]))
	meminfo.MemFree = meminfo.MemFree * pagesize

	// TODO swap free using kvm
	// cf. https://www.freebsd.org/cgi/man.cgi?query=kvm_getswapinfo&sektion=3
	// cf. libstatgrab (swap_stats.c)
	C.kvm_getswapinfo(kvmd, &swapinfo, 1, 0)

	meminfo.SwapFree = meminfo.SwapFree * pagesize

	return meminfo, nil
}
