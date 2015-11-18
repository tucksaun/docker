// +build !linux,!windows

package sandbox

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
}

// InitOSContext initializes OS context while configuring network resources
func InitOSContext() func() {
	return func() {}
}
