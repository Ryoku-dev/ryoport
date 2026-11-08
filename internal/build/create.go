package build

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

var specialCreate = map[string]bool{
	"windows": true, "windows-server": true, "macos": true,
	"freebsd": true, "ghostbsd": true, "dragonflybsd": true,
	"netbsd": true, "openbsd": true, "haiku": true, "freedos": true,
	"kolibrios": true, "openindiana": true, "reactos": true,
	"batocera": true, "slackware": true, "slax": true, "tinycore": true,
	"slitaz": true, "slint": true, "easyos": true, "netboot": true,
}

// Create downloads and configures one quickget catalogue machine.
func (b *Builder) Create(ctx context.Context, osName, release, edition string, emit func(Event)) (string, error) {
	if osName == "" || release == "" {
		return "", failEvent(emit, errors.New("create needs <os> <release> [edition]"))
	}
	name := osName + "-" + release
	if edition != "" {
		name += "-" + edition
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

	if !specialCreate[osName] {
		made, handled, err := b.createFast(ctx, name, osName, release, edition, emit)
		if handled {
			if err != nil {
				return "", failEvent(emit, err)
			}
			emitEvent(emit, Event{Phase: PhaseDone, Name: made})
			return made, nil
		}
	}
	made, err := b.createFallback(ctx, name, osName, release, edition, emit)
	if err != nil {
		return "", failEvent(emit, err)
	}
	emitEvent(emit, Event{Phase: PhaseDone, Name: made})
	return made, nil
}

func (b *Builder) createFast(ctx context.Context, name, osName, release, edition string, emit func(Event)) (string, bool, error) {
	emitEvent(emit, Event{Phase: PhaseResolve})
	args := []string{"--url", osName, release}
	if edition != "" {
		args = append(args, edition)
	}
	cmd := exec.CommandContext(ctx, "quickget", args...)
	cmd.Dir = b.VM.Root
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", true, ctx.Err()
		}
		return "", false, nil
	}
	downloadURL := firstHTTPURL(string(output))
	parsed, err := url.Parse(downloadURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false, nil
	}
	base := filepath.Base(parsed.Path)
	ext := strings.ToLower(filepath.Ext(base))
	if base == "." || (ext != ".iso" && ext != ".img") {
		return "", false, nil
	}
	stage := filepath.Join(b.VM.Root, ".staging-"+name)
	if err := os.RemoveAll(stage); err != nil {
		return "", true, err
	}
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return "", true, err
	}
	defer os.RemoveAll(stage)
	emitEvent(emit, Event{Phase: PhaseDownload, Name: name})
	dest := filepath.Join(stage, base)
	err = fetch.DownloadClient(ctx, b.client(), downloadURL, dest, func(progress fetch.Progress) {
		emitEvent(emit, Event{Phase: PhaseDownload, Name: name, Progress: &progress})
	})
	if err != nil {
		return "", true, fmt.Errorf("download failed: %w", err)
	}
	emitEvent(emit, Event{Phase: PhaseConfig})
	config := exec.CommandContext(ctx, "quickget", "--create-config", name, dest)
	config.Dir = b.VM.Root
	if output, err := config.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return "", true, ctx.Err()
		}
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", true, fmt.Errorf("could not write the VM config: %s", message)
	}
	if err := stampOS(b.VM.ConfPath(name), osName); err != nil {
		return "", true, err
	}
	return name, true, nil
}

func firstHTTPURL(output string) string {
	for _, field := range strings.Fields(output) {
		candidate := strings.Trim(field, "'\"()[]<>,")
		if strings.HasPrefix(candidate, "https://") || strings.HasPrefix(candidate, "http://") {
			return candidate
		}
	}
	return ""
}

func (b *Builder) createFallback(ctx context.Context, requested, osName, release, edition string, emit func(Event)) (string, error) {
	emitEvent(emit, Event{Phase: PhaseDownload, Name: requested})
	before, err := confNames(b.VM.Root)
	if err != nil {
		return "", err
	}
	_, dirErr := os.Stat(b.VM.Dir(requested))
	dirExisted := dirErr == nil || !errors.Is(dirErr, os.ErrNotExist)
	args := []string{"quickget", osName, release}
	if edition != "" {
		args = append(args, edition)
	}
	cmd := exec.CommandContext(ctx, "setsid", args...)
	cmd.Dir = b.VM.Root
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 3 * time.Second
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var lastLine string
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		lastLine = strings.TrimSuffix(scanner.Text(), "\r")
		if lastLine != "" {
			emitEvent(emit, Event{Phase: PhaseLog, Line: lastLine})
		}
	}
	waitErr := cmd.Wait()
	if scanErr := scanner.Err(); scanErr != nil && waitErr == nil {
		waitErr = scanErr
	}
	made, findErr := b.newConfigs(before, osName+"-"+release)
	if ctx.Err() != nil {
		b.cleanNewConfigs(made)
		if !dirExisted {
			_ = os.RemoveAll(b.VM.Dir(requested))
		}
		return "", ctx.Err()
	}
	if findErr != nil {
		return "", findErr
	}
	if len(made) == 0 {
		if lastLine != "" {
			return "", fmt.Errorf("download failed: %s", lastLine)
		}
		if waitErr != nil {
			return "", fmt.Errorf("download failed: %w", waitErr)
		}
		return "", errors.New("download failed")
	}
	sort.Strings(made)
	result := made[len(made)-1]
	for _, candidate := range made {
		if err := stampOS(b.VM.ConfPath(candidate), osName); err != nil {
			return "", err
		}
	}
	broken, guest, err := b.deadConfig(result)
	if err != nil {
		return "", err
	}
	if broken {
		b.cleanDeadConfig(result)
		if guest == "windows" || guest == "windows-server" {
			return "", errors.New("Windows downloads are blocked by Microsoft on this network.\nDownload the ISO in a browser, then use Load ISO")
		}
		return "", errors.New("the install media did not download; try again or load an ISO from disk")
	}
	if waitErr != nil {
		return "", fmt.Errorf("download failed: %s", lastLine)
	}
	if guest == "windows" || guest == "windows-server" {
		if err := setFixedISO(b.VM.ConfPath(result)); err != nil {
			return "", err
		}
		_ = os.Remove(filepath.Join(b.VM.Dir(result), "virtio-win.iso"))
	}
	return result, nil
}

func confNames(root string) (map[string]bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".conf" {
			out[strings.TrimSuffix(entry.Name(), ".conf")] = true
		}
	}
	return out, nil
}

func (b *Builder) newConfigs(before map[string]bool, prefix string) ([]string, error) {
	after, err := confNames(b.VM.Root)
	if err != nil {
		return nil, err
	}
	var out []string
	for name := range after {
		if !before[name] && strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out, nil
}

func (b *Builder) cleanNewConfigs(names []string) {
	for _, name := range names {
		_ = os.Remove(b.VM.ConfPath(name))
		_ = os.RemoveAll(b.VM.Dir(name))
	}
}

func stampOS(path, osName string) error {
	conf, err := qconf.Read(path)
	if err != nil {
		return err
	}
	if err := conf.Set("ryovm_os", osName); err != nil {
		return err
	}
	return conf.Write(path)
}

func setFixedISO(path string) error {
	conf, err := qconf.Read(path)
	if err != nil {
		return err
	}
	if err := conf.Set("fixed_iso", filepath.Join(xdg.ImagesDir(), "virtio-win.iso")); err != nil {
		return err
	}
	return conf.Write(path)
}

func (b *Builder) deadConfig(name string) (bool, string, error) {
	conf, err := qconf.Read(b.VM.ConfPath(name))
	if err != nil {
		return false, "", err
	}
	guest := conf.Get("guest_os")
	iso := conf.Get("iso")
	if iso == "" {
		return false, guest, nil
	}
	if !filepath.IsAbs(iso) {
		iso = filepath.Join(b.VM.Root, iso)
	}
	_, err = os.Stat(iso)
	if err == nil {
		return false, guest, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, guest, nil
	}
	return false, guest, err
}

func (b *Builder) cleanDeadConfig(name string) {
	disks, _ := filepath.Glob(filepath.Join(b.VM.Dir(name), "*.qcow2"))
	if len(disks) == 0 {
		_ = os.RemoveAll(b.VM.Dir(name))
	}
	_ = os.Remove(b.VM.ConfPath(name))
}
