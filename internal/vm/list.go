package vm

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/xdg"
	"golang.org/x/sys/unix"
)

// List returns every machine configuration in name order.
func (m *Manager) List(ctx context.Context) ([]Machine, error) {
	if err := xdg.EnsureDir(m.Root); err != nil {
		return nil, err
	}
	root, resolvedRoot, err := m.resolvedRoot()
	if err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(root, "*.conf"))
	if err != nil {
		return nil, err
	}
	machines := make([]Machine, 0, len(paths))
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".conf")
		if name == "" {
			continue
		}
		confPath, err := confPathForExistingName(name, root, resolvedRoot)
		if err != nil {
			machines = append(machines, machineDefaults(name, nil))
			continue
		}
		conf, err := qconf.Read(confPath)
		if err != nil {
			machines = append(machines, machineDefaults(name, nil))
			continue
		}
		machine, err := m.machine(name, conf)
		if err != nil {
			machine = machineDefaults(name, conf)
		}
		machines = append(machines, machine)
	}
	return machines, nil
}

// Get returns one machine with its paths, snapshots and SSH readiness.
func (m *Manager) Get(ctx context.Context, name string) (Detail, error) {
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return Detail{}, err
	}
	machine, err := m.machine(name, conf)
	if err != nil {
		return Detail{}, err
	}
	_, dir, _, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return Detail{}, err
	}
	snapshots, err := m.Snapshots(ctx, name)
	if err != nil {
		return Detail{}, err
	}
	machine.Sealed = false
	for _, snapshot := range snapshots {
		if snapshot.Name == "sealed" {
			machine.Sealed = true
			break
		}
	}
	user := conf.Get("ryovm_ssh_user")
	if user == "" {
		user = os.Getenv("SUDO_USER")
	}
	if user == "" {
		user = os.Getenv("USER")
	}
	return Detail{
		Machine:   machine,
		SSHReady:  machine.Running && machine.SSH != "" && sshReady(ctx, machine.SSH),
		SSHUser:   user,
		Snapshots: snapshots,
		Conf:      confPath,
		Dir:       dir,
	}, nil
}

func (m *Manager) machine(name string, conf *qconf.Conf) (Machine, error) {
	disk, dir, pid, ports, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return Machine{}, err
	}
	machine := machineDefaults(name, conf)
	machine.Running = running(pid)
	machine.DiskUsed = allocatedBytes(dir)
	machine.Installed = pathExists(disk)
	machine.Disposable = machine.Burn || (machine.Running && pathExists(filepath.Join(dir, name+".disposable")))
	machine.Sealed = conf.Get("ryovm_sealed") == "1" && machine.Installed
	if machine.Running {
		machine.SSH = portOf(ports, "ssh")
		machine.Spice = spiceTarget(ports, dir)
	}
	return machine, nil
}

func machineDefaults(name string, conf *qconf.Conf) Machine {
	machine := Machine{
		Name:    name,
		Guest:   "linux",
		Display: "gtk",
		Cores:   "auto",
		RAM:     "auto",
		UEFI:    true,
	}
	if conf == nil {
		return machine
	}
	if value := conf.Get("guest_os"); value != "" {
		machine.Guest = value
	}
	if value := conf.Get("display"); value != "" {
		machine.Display = value
	}
	if value := conf.Get("cpu_cores"); value != "" {
		machine.Cores = value
	}
	if value := conf.Get("ram"); value != "" {
		machine.RAM = value
	}
	machine.OS = conf.Get("ryovm_os")
	machine.Disk = conf.Get("disk_size")
	machine.Instant = conf.Has("ryovm_ssh_user") || conf.Has("ryovm_tools")
	machine.Burn = conf.Get("ryovm_burn") == "1"
	machine.Disposable = machine.Burn
	machine.TPM = conf.Get("tpm") == "on"
	machine.SecureBoot = conf.Get("secureboot") == "on"
	machine.UEFI = conf.Get("boot") != "legacy"
	return machine
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func allocatedBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err == nil {
			total += stat.Blocks * 512
		}
		return nil
	})
	return total
}

func sshReady(ctx context.Context, port string) bool {
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	banner := make([]byte, 4)
	if _, err = io.ReadFull(conn, banner); err != nil {
		return false
	}
	return string(banner) == "SSH-"
}
