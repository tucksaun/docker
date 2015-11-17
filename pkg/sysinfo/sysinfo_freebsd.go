package sysinfo

// TODO FreeBSD
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	sysInfo.cgroupMemInfo = &cgroupMemInfo{}
	sysInfo.cgroupCpuInfo = &cgroupCpuInfo{}
	return sysInfo
}
