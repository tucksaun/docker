package system

// MemInfo contains memory statistics of the host system.
type MemInfo struct {
	// Total usable RAM (i.e. physical RAM minus a few reserved bits and the
	// kernel binary code).
	MemTotal uint64

	// Amount of free memory.
	MemFree uint64

	// Total amount of swap space available.
	SwapTotal uint64

	// Amount of swap space that is currently unused.
	SwapFree uint64
}
