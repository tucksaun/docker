package mount

/*
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <sys/_iovec.h>
#include <sys/mount.h>
#include <sys/param.h>
*/
import "C"

import (
	"fmt"
	"errors"
	"strings"
	"syscall"
	"unsafe"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"os"
)

func allocateIOVecs(options []string) []C.struct_iovec {
	out := make([]C.struct_iovec, len(options))
	for i, option := range options {
		out[i].iov_base = unsafe.Pointer(C.CString(option))
		out[i].iov_len = C.size_t(len(option) + 1)
	}
	return out
}

func mount(device, target, mType string, flag uintptr, data string) error {
	if mType == "bind" {
		mType = "nullfs"
	} else {
		xs := strings.Split(data, ",")
		for _, x := range xs {
			if x == "bind" {
				mType = "nullfs"
				break
			}
		}
	}

	if mType == "nullfs" {
		if fileInfo, _ := os.Stat(device); !fileInfo.IsDir() {
			logrus.Warnf("Can't mount %q: not a directory", device)
			return nil
		}

		if out, err := exec.Command("mount_nullfs", device, target).Output(); err != nil {
			return errors.New(string(out))
		}

		return nil
	}

	options := []string{
		"fspath", target,
		"fstype", mType,
		"from", device,
	}
	rawOptions := allocateIOVecs(options)
	for _, rawOption := range rawOptions {
		defer C.free(rawOption.iov_base)
	}

	logrus.Debugf("C.nmount (%+v, %+v)", options, flag)

	if ret, err := C.nmount(&rawOptions[0], C.uint(len(options)), C.int(flag)); ret != 0 {
		errno := err.(syscall.Errno)
		return fmt.Errorf("Failed to call nmount: %s (%d)", err, int(errno))
	}
	return nil
}

func unmount(target string, flag int) error {
	return syscall.Unmount(target, flag)
}
