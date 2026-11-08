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
	paths, err := filepath.Glob(filepath.Join(m.Root, "*.conf"))
	if err != nil {
		return nil, err
	}
	machines := make([]Machine, 0, len(paths))
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".conf")
		conf, err := qconf.Read(path)
		if err != nil {
			return nil, err
		}
		machine, err := m.machine(name, conf)
		if err != nil {
			return nil, err
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
	isRunning := running(pid)
	display := conf.Get("display")
	if display == "" {
		display = "gtk"
	}
	cores := conf.Get("cpu_cores")
	if cores == "" {
		cores = "auto"
	}
	ram := conf.Get("ram")
	if ram == "" {
		ram = "auto"
	}
	guest := conf.Get("guest_os")
	if guest == "" {
		guest = "linux"
	}
	machine := Machine{
		Name:       name,
		Guest:      guest,
		OS:         conf.Get("ryovm_os"),
		Display:    display,
		Cores:      cores,
		RAM:        ram,
		Disk:       conf.Get("disk_size"),
		DiskUsed:   allocatedBytes(dir),
		Running:    isRunning,
		Installed:  pathExists(disk),
		Disposable: isRunning && pathExists(filepath.Join(dir, name+".disposable")),
		Sealed:     conf.Get("ryovm_sealed") == "1" && pathExists(disk),
		TPM:        conf.Get("tpm") == "on",
		SecureBoot: conf.Get("secureboot") == "on",
		UEFI:       conf.Get("boot") != "legacy",
	}
	if isRunning {
		machine.SSH = portOf(ports, "ssh")
		machine.Spice = spiceTarget(ports, dir)
	}
	return machine, nil
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
