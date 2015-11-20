package types

import (
	"os"
	"time"
)

// ContainerPathStat is used to encode the header from
// 	GET /containers/{name:.*}/archive
// "name" is basename of the resource.
type ContainerPathStat struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	Mtime      time.Time   `json:"mtime"`
	LinkTarget string      `json:"linkTarget"`
}
