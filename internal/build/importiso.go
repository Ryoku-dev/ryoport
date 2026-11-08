package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

// Import creates a quickemu config for local installation media.
func (b *Builder) Import(name, iso, guest, osName string) error {
	if !vm.ValidName(name) {
		return fmt.Errorf("invalid machine name %q", name)
	}
	info, err := os.Stat(iso)
	if err != nil {
		return fmt.Errorf("ISO not found: %s: %w", iso, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("ISO is not a regular file: %s", iso)
	}
	iso, err = filepath.Abs(iso)
	if err != nil {
		return err
	}
	if guest == "" {
		guest = "linux"
	}
	if osName == "" {
		osName = guest
	}
	if err := xdg.EnsureDir(b.VM.Root); err != nil {
		return err
	}
	path := b.VM.ConfPath(name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("a VM named %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	conf := qconf.Parse(nil)
	values := [][2]string{
		{"guest_os", guest},
		{"disk_img", name + "/disk.qcow2"},
		{"iso", iso},
		{"disk_size", "64G"},
		{"ryovm_os", osName},
	}
	if guest == "windows" || guest == "windows-server" {
		values = append(values,
			[2]string{"tpm", "on"},
			[2]string{"secureboot", "off"},
			[2]string{"fixed_iso", filepath.Join(xdg.ImagesDir(), "virtio-win.iso")},
		)
	}
	for _, item := range values {
		if err := conf.Set(item[0], item[1]); err != nil {
			return err
		}
	}
	return conf.Write(path)
}
