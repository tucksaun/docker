// +build !linux

package system

func ReadMemInfo() (*MemInfo, error) {
	return &MemInfo{}, ErrNotSupportedPlatform
}
