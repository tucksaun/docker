package jail

import (
	"fmt"
	"os"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/Sirupsen/logrus"
	"path/filepath"
	"crypto/md5"
	"io"
)

func (d *driver) setupMounts(c *execdriver.Command) (mountPoints, params []string) {
	root := c.Rootfs
	mounts := make(map[string]execdriver.Mount)
	h := md5.New()

	for _, m := range c.Mounts {
		fileInfo, err := os.Stat(m.Source)
		if err != nil {
			logrus.Errorf("[jail] impossible to mount %s: %s.", m.Source, err.Error())
			continue
		}
		dirMount := m
		if !fileInfo.IsDir() {
			parentDir := filepath.Dir(m.Source)
			h.Reset()
			io.WriteString(h, parentDir)
			hash := h.Sum(nil)
			parentDestination := fmt.Sprintf("/.dockerbinds/%x", hash)
			destination := filepath.Join(parentDestination, filepath.Base(m.Source))

			if fi, _ := os.Stat(root+m.Destination); fi != nil && (fi.IsDir() || fi.Size() == 0) {
				os.Remove(root+m.Destination)
			}
			if err := os.Symlink(destination, root+m.Destination); err != nil {
				logrus.Errorf("[jail] impossible to mount %s: %s.", m.Source, err.Error())
				continue
			}

			mount, alreadyPresent := mounts[parentDestination]

			if alreadyPresent {
				mount.Writable = mount.Writable || m.Writable
				mount.Private = mount.Private && m.Private
				mount.Slave = mount.Slave && m.Slave

				continue;
			} else {
				fileInfo, err = os.Stat(parentDir)
				dirMount = execdriver.Mount{
					Source: parentDir,
					Destination: parentDestination,
					Writable: m.Writable,
					Private: m.Private,
					Slave: m.Slave,
				}
			}
		}

		if err := os.MkdirAll(root + dirMount.Destination, fileInfo.Mode()); err != nil {
			logrus.Errorf("[jail] impossible to mount %s: %s.", dirMount.Source, err.Error())
			continue
		}

		mounts[dirMount.Destination] = dirMount
	}

	for _, m := range mounts {
		if m.Writable {
			params = append(params, fmt.Sprintf("mount=%s %s nullfs rw 0 0", m.Source, root+m.Destination))
		} else {
			params = append(params, fmt.Sprintf("mount=%s %s nullfs ro 0 0", m.Source, root+m.Destination))
		}
		mountPoints = append(mountPoints, m.Destination)
	}

	return
}
