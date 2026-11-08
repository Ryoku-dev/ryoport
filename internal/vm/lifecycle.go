package vm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

var sizePattern = regexp.MustCompile(`^[0-9]+[MG]$`)

// Launch starts quickemu in a new session and waits for its QEMU pid file.
func (m *Manager) Launch(ctx context.Context, name string, display Display, disposable bool) error {
	if _, err := exec.LookPath("quickemu"); err != nil {
		return errors.New("quickemu is not installed. Run setup first.")
	}
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	var quickemuDisplay string
	switch display {
	case "", DisplayWindow:
		quickemuDisplay = "gtk"
	case DisplaySpice:
		quickemuDisplay = "spice"
	case DisplayHeadless:
		quickemuDisplay = "none"
	default:
		return errors.New("launch display must be window|spice|headless")
	}
	_, dir, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("%s is already running", name)
	}
	if iso := conf.Get("iso"); iso != "" {
		if !filepath.IsAbs(iso) {
			iso = filepath.Clean(filepath.Join(m.Root, iso))
			if !withinRoot(m.Root, iso) {
				return errors.New("iso escapes the VM root")
			}
		}
		diskRef := conf.Get("disk_img")
		if diskRef != "" && !filepath.IsAbs(diskRef) {
			diskRef = filepath.Clean(filepath.Join(m.Root, diskRef))
			if !withinRoot(m.Root, diskRef) {
				return errors.New("disk_img escapes the VM root")
			}
		}
		if !pathExists(iso) && (diskRef == "" || !pathExists(diskRef)) {
			return fmt.Errorf("%s has no install media: its ISO never finished downloading. Re-create the machine, or import an ISO from disk.", name)
		}
	}
	if err := conf.Set("display", quickemuDisplay); err != nil {
		return err
	}
	if err := conf.Write(confPath); err != nil {
		return err
	}
	if err := xdg.EnsureDir(dir); err != nil {
		return err
	}
	marker := filepath.Join(dir, name+".disposable")
	if disposable {
		if err := os.WriteFile(marker, nil, 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	logPath := filepath.Join(dir, name+".launch.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	args := []string{"--vm", filepath.Base(confPath), "--display", quickemuDisplay}
	if quickemuDisplay != "spice" {
		args = append(args, "--viewer", "none")
	}
	if disposable {
		args = append(args, "--status-quo")
	}
	command := exec.Command("quickemu", args...)
	command.Dir = m.Root
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	if err := command.Process.Release(); err != nil {
		_ = logFile.Close()
		return err
	}
	if err := logFile.Close(); err != nil {
		return err
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for {
		if running(pid) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-timer.C:
			if running(pid) {
				return nil
			}
			return launchError(logPath)
		}
	}
}

func launchError(logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return errors.New("the VM failed to start")
	}
	defer file.Close()
	const tailSize = 12
	var tail [tailSize]string
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		tail[count%tailSize] = strings.TrimSuffix(scanner.Text(), "\r")
		count++
	}
	if count == 0 {
		return errors.New("the VM failed to start")
	}
	lineCount := min(count, tailSize)
	lines := make([]string, lineCount)
	for i := range lineCount {
		lines[i] = tail[(count-lineCount+i)%tailSize]
	}
	last := tail[(count-1)%tailSize]
	return fmt.Errorf("the VM failed to start: %s\n%s", last, strings.Join(lines, "\n"))
}

// Stop asks the guest to shut down, then falls back to quickemu --kill.
func (m *Manager) Stop(ctx context.Context, name string, force bool) error {
	if _, err := exec.LookPath("quickemu"); err != nil {
		return errors.New("quickemu is not installed. Run setup first.")
	}
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	_, dir, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	monitor := filepath.Join(dir, name+"-monitor.socket")
	if !force && running(pid) {
		if info, err := os.Stat(monitor); err == nil && info.Mode()&os.ModeSocket != 0 {
			_ = qmp.Power(ctx, monitor, qmp.PowerShutdown)
			if waitStopped(ctx, pid, 20*time.Second) {
				return nil
			}
		}
	}
	command := exec.CommandContext(ctx, "quickemu", "--vm", filepath.Base(confPath), "--kill")
	command.Dir = m.Root
	_ = command.Run()
	_ = waitStopped(ctx, pid, 5*time.Second)
	return nil
}

func waitStopped(ctx context.Context, pidFile string, timeout time.Duration) bool {
	if !running(pidFile) {
		return true
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if !running(pidFile) {
				return true
			}
		case <-timer.C:
			return !running(pidFile)
		}
	}
}

// OpenViewer opens the running machine's SPICE endpoint.
func (m *Manager) OpenViewer(name string) error {
	_, dir, pid, ports, err := m.runtimePaths(name)
	if err != nil {
		return err
	}
	if !running(pid) {
		return fmt.Errorf("%s is not running", name)
	}
	target := spiceTarget(ports, dir)
	if target == "" {
		return fmt.Errorf("%s has no SPICE console; relaunch it in SPICE or Headless mode", name)
	}
	var executable string
	var args []string
	if spicy, err := exec.LookPath("spicy"); err == nil {
		executable = spicy
		if strings.HasPrefix(target, "unix:") {
			args = []string{"--title", name, "--uri=spice+unix://" + strings.TrimPrefix(target, "unix:")}
		} else {
			args = []string{"--title", name, "--port", target}
		}
	} else if viewer, err := exec.LookPath("remote-viewer"); err == nil {
		executable = viewer
		if strings.HasPrefix(target, "unix:") {
			args = []string{"spice+unix://" + strings.TrimPrefix(target, "unix:")}
		} else {
			args = []string{"spice://localhost:" + target}
		}
	} else {
		return errors.New("no SPICE viewer found (install spice-gtk)")
	}
	command := exec.Command(executable, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}

// Delete removes a stopped machine, or only its disk image.
func (m *Manager) Delete(ctx context.Context, name string, diskOnly bool) error {
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	disk, dir, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before deleting it", name)
	}
	if !diskOnly && dir != filepath.Clean(m.Root) && !withinRoot(m.Root, dir) {
		return fmt.Errorf("refusing to delete VM directory outside %s", m.Root)
	}
	if _, err := exec.LookPath("quickemu"); err == nil {
		verb := "--delete-vm"
		if diskOnly {
			verb = "--delete-disk"
		}
		command := exec.CommandContext(ctx, "quickemu", "--vm", filepath.Base(confPath), verb)
		command.Dir = m.Root
		_ = command.Run()
	}
	if diskOnly {
		if err := os.Remove(disk); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if dir != filepath.Clean(m.Root) {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.Remove(confPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func withinRoot(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Rename moves a stopped machine and rewrites its relative image paths.
func (m *Manager) Rename(old, new string) error {
	if !ValidName(new) {
		return fmt.Errorf("invalid name: '%s'", new)
	}
	conf, confPath, err := m.readConf(old)
	if err != nil {
		return err
	}
	_, _, pid, _, err := m.runtimePathsFor(old, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before renaming it", old)
	}
	newConfPath := m.ConfPath(new)
	if pathExists(newConfPath) {
		return fmt.Errorf("a VM named '%s' already exists", new)
	}
	newDir := m.Dir(new)
	if pathExists(newDir) {
		return fmt.Errorf("%s already exists", newDir)
	}
	for _, key := range []string{"disk_img", "iso"} {
		value := conf.Get(key)
		if strings.HasPrefix(value, old+"/") {
			if err := conf.Set(key, new+strings.TrimPrefix(value, old)); err != nil {
				return err
			}
		}
	}
	if err := conf.Write(confPath); err != nil {
		return err
	}
	oldDir := m.Dir(old)
	if info, statErr := os.Stat(oldDir); statErr == nil && info.IsDir() {
		if err := os.Rename(oldDir, newDir); err != nil {
			return err
		}
		entries, err := os.ReadDir(newDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			base := entry.Name()
			if !strings.HasPrefix(base, old+".") && !strings.HasPrefix(base, old+"-") {
				continue
			}
			if err := os.Rename(filepath.Join(newDir, base), filepath.Join(newDir, new+strings.TrimPrefix(base, old))); err != nil {
				return err
			}
		}
	}
	return os.Rename(confPath, newConfPath)
}

// Resize grows a stopped machine disk and records the new capacity.
func (m *Manager) Resize(ctx context.Context, name, size string) error {
	if !sizePattern.MatchString(size) {
		return errors.New("size must look like 64G or 4096M")
	}
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	disk, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before resizing its disk", name)
	}
	if !pathExists(disk) {
		return fmt.Errorf("%s has no disk yet; launch it once first", name)
	}
	if err := exec.CommandContext(ctx, "qemu-img", "resize", disk, size).Run(); err != nil {
		return fmt.Errorf("could not grow the disk to %s", size)
	}
	if err := conf.Set("disk_size", size); err != nil {
		return err
	}
	return conf.Write(confPath)
}

// SSHCommand returns the exact argv used for a VM login.
func (m *Manager) SSHCommand(name string) ([]string, error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return nil, err
	}
	_, dir, pid, ports, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return nil, err
	}
	if !running(pid) {
		return nil, fmt.Errorf("%s is not running", name)
	}
	port := portOf(ports, "ssh")
	if port == "" {
		return nil, fmt.Errorf("%s has no forwarded SSH port", name)
	}
	argv := []string{"env", "TERM=xterm-256color", "ssh"}
	if conf.Get("ryovm_burn") == "1" {
		argv = append(argv,
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "StrictHostKeyChecking=no",
			"-o", "LogLevel=ERROR",
		)
	} else {
		argv = append(argv,
			"-o", "UserKnownHostsFile="+filepath.Join(dir, name+".known_hosts"),
			"-o", "StrictHostKeyChecking=accept-new",
		)
	}
	argv = append(argv, "-o", "ConnectTimeout=8")
	if burnKey := xdg.BurnKey(); pathExists(burnKey) {
		argv = append(argv, "-i", burnKey)
	}
	user := conf.Get("ryovm_ssh_user")
	if user == "" {
		user = os.Getenv("SUDO_USER")
	}
	if user == "" {
		user = os.Getenv("USER")
	}
	argv = append(argv, "-p", port, user+"@localhost")
	return argv, nil
}

// Folder returns the machine directory, or the root when name is empty.
func (m *Manager) Folder(name string) string {
	if name == "" {
		return m.Root
	}
	if disk, err := m.DiskPath(name); err == nil {
		return filepath.Dir(disk)
	}
	return m.Dir(name)
}
