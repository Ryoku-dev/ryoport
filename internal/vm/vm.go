// Package vm manages quickemu machines and their compatible on-disk state.
package vm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/xdg"
	"golang.org/x/sys/unix"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Display selects how quickemu exposes a machine.
type Display string

const (
	DisplayWindow   Display = "window"
	DisplaySpice    Display = "spice"
	DisplayHeadless Display = "headless"
)

// Machine matches one object from ryovm list.
type Machine struct {
	Name       string `json:"name"`
	Guest      string `json:"guest"`
	OS         string `json:"os"`
	Display    string `json:"display"`
	Cores      string `json:"cores"`
	RAM        string `json:"ram"`
	Disk       string `json:"disk"`
	DiskUsed   int64  `json:"diskUsed"`
	Running    bool   `json:"running"`
	Installed  bool   `json:"installed"`
	SSH        string `json:"ssh"`
	Spice      string `json:"spice"`
	Disposable bool   `json:"disposable"`
	Sealed     bool   `json:"sealed"`
	TPM        bool   `json:"tpm"`
	SecureBoot bool   `json:"secureboot"`
	UEFI       bool   `json:"uefi"`
}

// Snapshot is a named qcow2 snapshot.
type Snapshot struct {
	Name string `json:"name"`
	Date string `json:"date"`
}

// Detail matches the object printed by ryovm get.
type Detail struct {
	Machine
	SSHReady  bool       `json:"sshReady"`
	SSHUser   string     `json:"sshUser"`
	Snapshots []Snapshot `json:"snapshots"`
	Conf      string     `json:"conf"`
	Dir       string     `json:"dir"`
}

// Manager operates on one quickemu machine root.
type Manager struct {
	Root string
}

// New returns a manager for the compatible RYOVM_HOME location.
func New() *Manager { return &Manager{Root: xdg.VMRoot()} }

// ValidName reports whether name is safe as one machine path component.
func ValidName(name string) bool {
	return namePattern.MatchString(name) && name != "." && name != ".."
}

// ConfPath returns the conventional machine configuration path.
func (m *Manager) ConfPath(name string) string { return filepath.Join(m.Root, name+".conf") }

// Dir returns the conventional per-machine directory.
func (m *Manager) Dir(name string) string { return filepath.Join(m.Root, name) }

func (m *Manager) resolvedRoot() (root, resolved string, err error) {
	root, err = filepath.Abs(m.Root)
	if err != nil {
		return "", "", err
	}
	resolved, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	return root, resolved, nil
}

func confPathForExistingName(name, root, resolvedRoot string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name: '%s'", name)
	}
	path := filepath.Join(root, name+".conf")
	if !withinRoot(root, path) {
		return "", fmt.Errorf("invalid name: '%s'", name)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !withinRoot(resolvedRoot, resolvedPath) {
		return "", fmt.Errorf("VM config for '%s' escapes the VM root", name)
	}
	return path, nil
}

func (m *Manager) existingConfPath(name string) (string, error) {
	root, resolvedRoot, err := m.resolvedRoot()
	if err != nil {
		return "", err
	}
	return confPathForExistingName(name, root, resolvedRoot)
}

// DiskPath resolves disk_img exactly as quickemu does for the compatible root.
func (m *Manager) DiskPath(name string) (string, error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return "", err
	}
	return m.resolveDisk(name, conf)
}

func (m *Manager) resolveDisk(name string, conf *qconf.Conf) (string, error) {
	disk := conf.Get("disk_img")
	if disk == "" {
		disk = filepath.Join(name, "disk.qcow2")
	}
	if filepath.IsAbs(disk) {
		return filepath.Clean(disk), nil
	}
	resolved := filepath.Clean(filepath.Join(m.Root, disk))
	relative, err := filepath.Rel(filepath.Clean(m.Root), resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("disk_img escapes the VM root")
	}
	return resolved, nil
}

// Sockets returns the monitor, guest-agent and pid paths for a machine.
func (m *Manager) Sockets(name string) (monitor, agent, pid string) {
	dir := m.Dir(name)
	if disk, err := m.DiskPath(name); err == nil {
		dir = filepath.Dir(disk)
	}
	return filepath.Join(dir, name+"-monitor.socket"), filepath.Join(dir, name+"-agent.sock"), filepath.Join(dir, name+".pid")
}

func (m *Manager) readConf(name string) (*qconf.Conf, string, error) {
	path, err := m.existingConfPath(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("no VM named '%s'", name)
	}
	if err != nil {
		return nil, "", err
	}
	conf, err := qconf.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("no VM named '%s'", name)
	}
	if err != nil {
		return nil, "", err
	}
	return conf, path, nil
}

func (m *Manager) runtimePaths(name string) (disk, dir, pid, ports string, err error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return "", "", "", "", err
	}
	return m.runtimePathsFor(name, conf)
}

func (m *Manager) runtimePathsFor(name string, conf *qconf.Conf) (disk, dir, pid, ports string, err error) {
	disk, err = m.resolveDisk(name, conf)
	if err != nil {
		return "", "", "", "", err
	}
	dir = filepath.Dir(disk)
	pid = filepath.Join(dir, name+".pid")
	ports = filepath.Join(dir, name+".ports")
	return disk, dir, pid, ports, nil
}

func running(pidFile string) bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	err = unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}

func portOf(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + ","
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if port := leadingDecimal(strings.TrimPrefix(line, prefix)); port != "" {
			return port
		}
	}
	return ""
}

func spiceTarget(path, dir string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "spice,") {
			if port := leadingDecimal(strings.TrimPrefix(line, "spice,")); port != "" {
				return port
			}
		}
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "unix,") {
			continue
		}
		socket := strings.TrimPrefix(line, "unix,")
		if filepath.IsAbs(socket) {
			return "unix:" + socket
		}
		return "unix:" + filepath.Join(dir, socket)
	}
	return ""
}

func leadingDecimal(value string) string {
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	return value[:end]
}
