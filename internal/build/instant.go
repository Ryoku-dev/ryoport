package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

// InstantOptions controls a preinstalled cloud machine build.
type InstantOptions struct {
	OS         string
	Name       string
	Disposable bool
	Sealed     bool
	Tools      []string
	Pkgs       []string
}

// Instant creates a cloud-image overlay, provisions it, and launches it headlessly.
func (b *Builder) Instant(ctx context.Context, options InstantOptions, emit func(Event)) (string, error) {
	var image CloudImage
	found := false
	for _, candidate := range Cloud() {
		if candidate.OS == options.OS {
			image, found = candidate, true
			break
		}
	}
	if !found {
		return "", failEvent(emit, fmt.Errorf("no cloud image named %q", options.OS))
	}
	name := options.Name
	if name == "" {
		name = options.OS + "-instant"
	}
	if !vm.ValidName(name) {
		return "", failEvent(emit, fmt.Errorf("invalid machine name %q", name))
	}
	if err := xdg.EnsureDir(b.VM.Root); err != nil {
		return "", failEvent(emit, err)
	}
	if _, err := os.Stat(b.VM.ConfPath(name)); err == nil {
		return "", failEvent(emit, fmt.Errorf("%s already exists", name))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", failEvent(emit, err)
	}
	imageURL, err := b.ResolveCloud(ctx, image)
	if err != nil {
		return "", failEvent(emit, fmt.Errorf("could not resolve the image URL: %w", err))
	}
	if err := xdg.EnsureDir(xdg.ImagesDir()); err != nil {
		return "", failEvent(emit, err)
	}
	base := filepath.Join(xdg.ImagesDir(), options.OS+"-cloud.qcow2")
	if info, statErr := os.Lstat(base); statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		part := base + ".part"
		defer os.Remove(part)
		emitEvent(emit, Event{Phase: PhaseDownload, Name: name})
		err = fetch.DownloadClient(ctx, b.client(), imageURL, part, func(progress fetch.Progress) {
			emitEvent(emit, Event{Phase: PhaseDownload, Name: name, Progress: &progress})
		})
		if err != nil {
			return "", failEvent(emit, fmt.Errorf("image download failed: %w", err))
		}
		if err := os.Rename(part, base); err != nil {
			return "", failEvent(emit, err)
		}
	}

	emitEvent(emit, Event{Phase: PhaseConfig})
	dir := b.VM.Dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", failEvent(emit, err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
			_ = os.Remove(b.VM.ConfPath(name))
		}
	}()
	disk := filepath.Join(dir, "disk.qcow2")
	if output, err := exec.CommandContext(ctx, "qemu-img", "create", "-f", "qcow2", "-F", "qcow2", "-b", base, disk, "32G").CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return "", failEvent(emit, ctx.Err())
		}
		return "", failEvent(emit, fmt.Errorf("could not create the disk overlay: %s", strings.TrimSpace(string(output))))
	}
	if err := b.writeSeed(ctx, dir, name, image.Sudo, options.OS, options.Tools, options.Pkgs, true); err != nil {
		return "", failEvent(emit, fmt.Errorf("could not build the cloud-init seed: %w", err))
	}
	conf := qconf.Parse(nil)
	settings := [][2]string{
		{"guest_os", "linux"},
		{"disk_img", name + "/disk.qcow2"},
		{"preallocation", "metadata"},
		{"extra_args", "-drive file=" + filepath.Join(dir, "seed.iso") + ",media=cdrom,readonly=on"},
		{"ryovm_os", options.OS},
		{"ryovm_ssh_user", "ryoku"},
		{"ryovm_burn", "1"},
	}
	if len(options.Tools) > 0 {
		settings = append(settings, [2]string{"ryovm_tools", strings.Join(options.Tools, ",")})
	}
	if options.Sealed {
		settings = append(settings, [2]string{"ryovm_sealed", "1"})
	}
	for _, setting := range settings {
		if err := conf.Set(setting[0], setting[1]); err != nil {
			return "", failEvent(emit, err)
		}
	}
	if err := conf.Write(b.VM.ConfPath(name)); err != nil {
		return "", failEvent(emit, err)
	}
	if options.Sealed {
		if output, err := exec.CommandContext(ctx, "qemu-img", "snapshot", "-c", "sealed", disk).CombinedOutput(); err != nil {
			return "", failEvent(emit, fmt.Errorf("could not seal the cloud disk: %s", strings.TrimSpace(string(output))))
		}
	}
	complete = true
	emitEvent(emit, Event{Phase: PhaseDone, Name: name})
	if err := b.VM.Launch(ctx, name, vm.DisplayHeadless, options.Disposable); err != nil {
		return name, err
	}
	return name, nil
}
