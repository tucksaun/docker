// +build !linux,!freebsd,!darwin

package system

func ReadMemInfo() (*MemInfo, error) {
	return &MemInfo{}, ErrNotSupportedPlatform
}
