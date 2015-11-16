package system

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

/*
#include <mach/mach_host.h>
*/
import "C"

type xsw_usage struct {
	Total, Avail, Used uint64
}

// ReadMemInfo retrieves memory statistics of the host system and returns a
//  MemInfo type.
func ReadMemInfo() (*MemInfo, error) {
	meminfo := &MemInfo{}

	var (
		val string
		err error
	)

	if val, err = syscall.Sysctl("hw.memsize"); err != nil {
		return nil, err
	}
	buf := []byte(val)

	meminfo.MemTotal = *(*uint64)(unsafe.Pointer(&buf[0]))
	var vmstat C.vm_statistics_data_t
	if err := vm_info(&vmstat); err != nil {
		return nil, err
	}

	meminfo.MemFree = uint64(vmstat.free_count) << 12

	sw_usage := xsw_usage{}

	if err := sysctlbyname("vm.swapusage", &sw_usage); err != nil {
		return nil, err
	}

	meminfo.SwapTotal = sw_usage.Total
	meminfo.SwapFree = sw_usage.Avail

	return meminfo, nil
}

func vm_info(vmstat *C.vm_statistics_data_t) error {
	var count C.mach_msg_type_number_t = C.HOST_VM_INFO_COUNT

	status := C.host_statistics(
		C.host_t(C.mach_host_self()),
		C.HOST_VM_INFO,
		C.host_info_t(unsafe.Pointer(vmstat)),
		&count)

	if status != C.KERN_SUCCESS {
		return fmt.Errorf("host_statistics=%d", status)
	}

	return nil
}

// generic Sysctl buffer unmarshalling
func sysctlbyname(name string, data interface{}) (err error) {
	val, err := syscall.Sysctl(name)
	if err != nil {
		return err
	}

	buf := []byte(val)

	switch v := data.(type) {
	case *uint64:
		*v = *(*uint64)(unsafe.Pointer(&buf[0]))
		return
	}

	bbuf := bytes.NewBuffer([]byte(val))
	return binary.Read(bbuf, binary.LittleEndian, data)
}
