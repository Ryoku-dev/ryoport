package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

// Template is the on-disk golden-image metadata format.
type Template struct {
	Name  string `json:"name"`
	OS    string `json:"os"`
	Tools string `json:"tools"`
	Base  string `json:"base"`
}

type templateMetadata struct {
	OS    string `json:"os"`
	Tools string `json:"tools"`
	Base  string `json:"base"`
}

// MakeTemplate flattens a machine disk into an immutable golden image.
func (b *Builder) MakeTemplate(ctx context.Context, source, name string) error {
	if !vm.ValidName(source) || !vm.ValidName(name) {
		return errors.New("invalid source or template name")
	}
	conf, err := qconf.Read(b.VM.ConfPath(source))
	if err != nil {
		return err
	}
	disk, err := b.VM.DiskPath(source)
	if err != nil {
		return err
	}
	if machineRunning(filepath.Join(filepath.Dir(disk), source+".pid")) {
		return fmt.Errorf("stop %s before templating it", source)
	}
	if info, err := os.Stat(disk); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s has no disk yet", source)
	}
	if err := xdg.EnsureDir(xdg.TemplatesDir()); err != nil {
		return err
	}
	base := filepath.Join(xdg.TemplatesDir(), name+".qcow2")
	metadata := filepath.Join(xdg.TemplatesDir(), name+".json")
	if _, err := os.Stat(base); err == nil {
		return fmt.Errorf("a template named %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	part := base + ".part"
	if err := os.Remove(part); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	defer os.Remove(part)
	output, err := exec.CommandContext(ctx, "qemu-img", "convert", "-O", "qcow2", disk, part).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not build the template: %s", strings.TrimSpace(string(output)))
	}
	if err := os.Chmod(part, 0o444); err != nil {
		return err
	}
	if err := os.Rename(part, base); err != nil {
		return err
	}
	meta := templateMetadata{OS: conf.Get("ryovm_os"), Tools: conf.Get("ryovm_tools"), Base: base}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := atomicWrite(metadata, append(data, '\n'), 0o644); err != nil {
		_ = os.Remove(base)
		return err
	}
	return nil
}

func Templates() ([]Template, error) {
	if err := xdg.EnsureDir(xdg.TemplatesDir()); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(xdg.TemplatesDir(), "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]Template, 0, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var meta templateMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("read template %s: %w", filepath.Base(path), err)
		}
		out = append(out, Template{Name: strings.TrimSuffix(filepath.Base(path), ".json"), OS: meta.OS, Tools: meta.Tools, Base: meta.Base})
	}
	return out, nil
}

func machineRunning(pidFile string) bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	err = syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Spawn creates a thin overlay from a golden template and launches it.
func (b *Builder) Spawn(ctx context.Context, tmpl, name string, disposable bool) (string, error) {
	if !vm.ValidName(tmpl) {
		return "", fmt.Errorf("invalid template name %q", tmpl)
	}
	if name == "" {
		name = fmt.Sprintf("%s-%04d", tmpl, time.Now().Unix()%10000)
	}
	if !vm.ValidName(name) {
		return "", fmt.Errorf("invalid machine name %q", name)
	}
	data, err := os.ReadFile(filepath.Join(xdg.TemplatesDir(), tmpl+".json"))
	if err != nil {
		return "", fmt.Errorf("no template %q: %w", tmpl, err)
	}
	var meta templateMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", err
	}
	base, err := filepath.Abs(meta.Base)
	if err != nil {
		return "", err
	}
	templateRoot, err := filepath.Abs(xdg.TemplatesDir())
	if err != nil {
		return "", err
	}
	if filepath.Dir(base) != templateRoot {
		return "", errors.New("template base is outside the template directory")
	}
	baseInfo, err := os.Lstat(base)
	if err != nil || !baseInfo.Mode().IsRegular() || baseInfo.Size() == 0 {
		return "", errors.New("template base is missing")
	}
	if err := xdg.EnsureDir(b.VM.Root); err != nil {
		return "", err
	}
	if _, err := os.Stat(b.VM.ConfPath(name)); err == nil {
		return "", fmt.Errorf("%s already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	dir := b.VM.Dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
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
		return "", fmt.Errorf("overlay failed: %s", strings.TrimSpace(string(output)))
	}
	if err := b.writeSeed(ctx, dir, name, false, meta.OS, nil, nil, false); err != nil {
		return "", fmt.Errorf("seed failed: %w", err)
	}
	conf := qconf.Parse(nil)
	settings := [][2]string{
		{"guest_os", "linux"},
		{"disk_img", name + "/disk.qcow2"},
		{"preallocation", "metadata"},
		{"extra_args", "-drive file=" + filepath.Join(dir, "seed.iso") + ",media=cdrom,readonly=on"},
		{"ryovm_os", meta.OS},
		{"ryovm_ssh_user", "ryoku"},
		{"ryovm_burn", "1"},
		{"ryovm_template", tmpl},
	}
	for _, setting := range settings {
		if err := conf.Set(setting[0], setting[1]); err != nil {
			return "", err
		}
	}
	if err := conf.Write(b.VM.ConfPath(name)); err != nil {
		return "", err
	}
	complete = true
	if err := b.VM.Launch(ctx, name, vm.DisplayHeadless, disposable); err != nil {
		return name, err
	}
	return name, nil
}
