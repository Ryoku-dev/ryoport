package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const virtioMinimumSize = 50 << 20

var virtioMirrors = []string{
	"https://fedora-virt.repo.nfrance.com/virtio-win/direct-downloads/stable-virtio/virtio-win.iso",
	"https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso",
}

// VirtIO fetches the shared Windows driver CD and optionally attaches it to a machine.
func (b *Builder) VirtIO(ctx context.Context, name string, emit func(Event)) (string, error) {
	dest := filepath.Join(xdg.ImagesDir(), "virtio-win.iso")
	emitEvent(emit, Event{Phase: PhaseDownload, Name: "virtio-win"})
	if !validISO(dest) {
		if err := xdg.EnsureDir(filepath.Dir(dest)); err != nil {
			return "", failEvent(emit, err)
		}
		part := dest + ".part"
		defer os.Remove(part)
		var mirrorErrors []error
		for _, mirror := range virtioMirrors {
			err := fetch.DownloadClient(ctx, b.client(), mirror, part, func(progress fetch.Progress) {
				emitEvent(emit, Event{Phase: PhaseDownload, Name: "virtio-win", Progress: &progress})
			})
			if err == nil && validISO(part) {
				if err = os.Rename(part, dest); err == nil {
					mirrorErrors = nil
					break
				}
			}
			if ctx.Err() != nil {
				return "", failEvent(emit, ctx.Err())
			}
			if err == nil {
				err = errors.New("download was not a valid ISO image")
			}
			mirrorErrors = append(mirrorErrors, fmt.Errorf("%s: %w", mirror, err))
			_ = os.Remove(part)
		}
		if len(mirrorErrors) > 0 {
			return "", failEvent(emit, fmt.Errorf("could not fetch the VirtIO driver CD: %w", errors.Join(mirrorErrors...)))
		}
	}
	if name != "" {
		path := b.VM.ConfPath(name)
		conf, err := qconf.Read(path)
		if err != nil {
			return "", failEvent(emit, err)
		}
		if err := conf.Set("fixed_iso", dest); err != nil {
			return "", failEvent(emit, err)
		}
		if err := conf.Write(path); err != nil {
			return "", failEvent(emit, err)
		}
		_ = os.Remove(filepath.Join(b.VM.Dir(name), "virtio-win.iso"))
	}
	emitEvent(emit, Event{Phase: PhaseDone, Name: "virtio-win"})
	return dest, nil
}

func validISO(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < virtioMinimumSize {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	marker := make([]byte, 5)
	if _, err := file.ReadAt(marker, 0x8001); err != nil && err != io.EOF {
		return false
	}
	return string(marker) == "CD001"
}
