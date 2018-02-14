package jail

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/mount"
	"github.com/Sirupsen/logrus"
)

const SPECIAL_MOUNT_DIR = "/.dockerbinds"
const MNAMELEN = 88

func (d *driver) setupMounts(c *execdriver.Command) (mountPoints, params []string) {
	root := c.Rootfs
	hasSpecialMounts := false
	mounts := make(map[string]execdriver.Mount)
	h := md5.New()

	for _, m := range c.Mounts {
		fileInfo, err := os.Stat(m.Source)
		if err != nil {
			logrus.Errorf("[jail] impossible to mount %s: %s.", m.Source, err.Error())
			continue
		}
		dirMount := m

		if originalDestination := filepath.Join(root, m.Destination); len(originalDestination) >= MNAMELEN || !fileInfo.IsDir() {
			h.Reset()
			target := m.Source
			logrus.Debugf("[mountbind] %s to %s (%s).", m.Source, m.Destination, originalDestination)

			if !fileInfo.IsDir() {
				target = filepath.Dir(m.Source)
			}

			io.WriteString(h, target)
			hash := h.Sum(nil)
			parentDestination := filepath.Join(SPECIAL_MOUNT_DIR, fmt.Sprintf("%x", hash))
			logrus.Debugf("[mountbind] target: %s", parentDestination)

			destination := parentDestination
			if !fileInfo.IsDir() {
				destination = filepath.Join(destination, filepath.Base(m.Source))
			}

			if fi, _ := os.Lstat(originalDestination); fi != nil {
				// directories, links and empty files can be removed
				logrus.Debugf("[mountbind] originalDestination already exists")
				if fi.IsDir() {
					// Special case where it overlaps with CWD
					if strings.HasPrefix(c.WorkingDir, m.Destination) {
						logrus.Debugf("[mountbind] originalDestination overlaps with CWD, we need to clean")
						os.RemoveAll(originalDestination)
					} else {
						syscall.Rmdir(originalDestination)
					}
				} else if fi.Mode() & os.ModeSymlink != 0 || fi.Size() == 0 {
					syscall.Unlink(originalDestination)
				}
			} else {
				logrus.Debugf("[mountbind] creating tree: %s", filepath.Dir(originalDestination))
				if err := os.MkdirAll(filepath.Dir(originalDestination), 0777); err != nil {
					logrus.Debugf("[mountbind] error while creating tree: %s", err)
				}
			}
			logrus.Debugf("[mountbind] link %s -> %s", originalDestination, destination)
			if err := os.Symlink(destination, originalDestination); err != nil {
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
				fileInfo, err = os.Stat(target)
				dirMount = execdriver.Mount{
					Source: target,
					Destination: parentDestination,
					Writable: m.Writable,
					Private: m.Private,
					Slave: m.Slave,
				}
			}

			hasSpecialMounts = true
		}

		if err := os.MkdirAll(filepath.Join(root, dirMount.Destination), fileInfo.Mode()); err != nil {
			logrus.Errorf("[jail] impossible to mount %s: %s.", dirMount.Source, err.Error())
			continue
		}

		mounts[dirMount.Destination] = dirMount
	}

	for _, m := range mounts {
		if m.Writable {
			params = append(params, fmt.Sprintf("mount=%s %s nullfs rw 0 0", m.Source, filepath.Join(root, m.Destination)))
		} else {
			params = append(params, fmt.Sprintf("mount=%s %s nullfs ro 0 0", m.Source, filepath.Join(root, m.Destination)))
		}
		mountPoints = append(mountPoints, m.Destination)
	}

	if hasSpecialMounts {
		os.Chmod(filepath.Join(root, SPECIAL_MOUNT_DIR), 0555)
	}

	return
}

func (d *driver) unsetupMounts(c *execdriver.Command, mountPoints []string) {
	hasSpecialMounts := false

	for _, mountpoint := range mountPoints {
		if err := mount.ForceUnmount(filepath.Join(c.Rootfs, mountpoint)); err != nil {
			logrus.Debugf("umount %s failed for %s: %s", c.ID, mountpoint, err)
		}
		if strings.HasPrefix(mountpoint, SPECIAL_MOUNT_DIR) {
			hasSpecialMounts = true
			syscall.Rmdir(filepath.Join(c.Rootfs, mountpoint))
		}
	}

	if hasSpecialMounts {
		syscall.Rmdir(filepath.Join(c.Rootfs, SPECIAL_MOUNT_DIR))
	}
}
