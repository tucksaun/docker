package daemon

// We can't bind mount files on FreeBSD
// instead we bind mount the parent,
// so we need to make it reachable to access hostname, hosts and resolv.conf

const DEFAULT_ROOT_FS_MASK = 0755