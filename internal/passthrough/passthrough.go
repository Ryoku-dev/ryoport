// Package passthrough manages Ryoku GPU-passthrough machines.
package passthrough

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var availability struct {
	sync.Once
	ok bool
}

type Readiness struct {
	Ready   bool   `json:"ready"`
	Blocker string `json:"blocker,omitempty"`
}

type Machine struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	OS     string `json:"os"`
	VCPUs  int    `json:"vcpus"`
	RAMMB  int    `json:"ramMb"`
	DiskGB int    `json:"diskGb"`
}

type List struct {
	Ready    bool      `json:"ready"`
	Blocker  string    `json:"blocker,omitempty"`
	Machines []Machine `json:"machines"`
}

type CreateOptions struct {
	Name   string
	ISO    string
	OS     string
	DiskGB int
	RAMMB  int
}

type Stats struct {
	Running         bool   `json:"running"`
	CPUTimeNS       uint64 `json:"cpuTimeNs"`
	MemoryKB        uint64 `json:"memoryKb"`
	DiskReadBytes   uint64 `json:"diskReadBytes"`
	DiskWriteBytes  uint64 `json:"diskWriteBytes"`
	NetReceiveBytes uint64 `json:"netRxBytes"`
	NetSendBytes    uint64 `json:"netTxBytes"`
}

func (stats Stats) MarshalJSON() ([]byte, error) {
	if !stats.Running {
		return []byte(`{"running":false}`), nil
	}
	type encoded Stats
	return json.Marshal(encoded(stats))
}

func Available() bool {
	availability.Do(func() {
		if _, err := exec.LookPath("ryoku-hub"); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := RunHub(ctx, "list")
		if err != nil {
			return
		}
		var list List
		availability.ok = json.Unmarshal(out, &list) == nil
	})
	return availability.ok
}

func Status(ctx context.Context) (List, error) {
	out, err := RunHub(ctx, "list")
	if err != nil {
		return List{}, err
	}
	var list List
	if err := json.Unmarshal(out, &list); err != nil {
		return List{}, fmt.Errorf("decode passthrough machine list: %w", err)
	}
	if list.Machines == nil {
		list.Machines = []Machine{}
	}
	return list, nil
}

func Create(ctx context.Context, o CreateOptions) error {
	if err := validateCreate(o); err != nil {
		return err
	}
	_, err := RunHub(ctx, "create", "--name", o.Name, "--iso", o.ISO, "--os", o.OS,
		"--disk-gb", strconv.Itoa(o.DiskGB), "--ram-mb", strconv.Itoa(o.RAMMB))
	return err
}

func Start(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := RunHub(ctx, "start", "--name", name)
	return err
}

func Stop(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	_, err := RunHub(ctx, "stop", "--name", name)
	return err
}

func Remove(ctx context.Context, name string, deleteDisk bool) error {
	if err := validateName(name); err != nil {
		return err
	}
	args := []string{"remove", "--name", name}
	if deleteDisk {
		args = append(args, "--delete-disk", "true")
	}
	_, err := RunHub(ctx, args...)
	return err
}

func ReadStats(ctx context.Context, name string) (Stats, error) {
	if err := validateName(name); err != nil {
		return Stats{}, err
	}
	virsh, err := lookupCommand("virsh")
	if err != nil {
		return Stats{}, errors.New("virsh is not installed")
	}
	domain := "ryoku-" + name
	stateOut, err := commandOutput(ctx, virsh, "--connect", "qemu:///system", "domstate", domain)
	if err != nil {
		return Stats{Running: false}, nil
	}
	state := strings.TrimSpace(string(stateOut))
	if state != "running" && state != "paused" {
		return Stats{Running: false}, nil
	}
	out, err := commandOutput(ctx, virsh, "--connect", "qemu:///system", "domstats", domain,
		"--cpu-total", "--balloon", "--block", "--interface", "--raw")
	if err != nil {
		return Stats{}, fmt.Errorf("could not read live metrics for %s: %w", name, err)
	}
	stats := Stats{Running: true}
	for _, line := range strings.Split(string(out), "\n") {
		key, raw, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			continue
		}
		switch {
		case key == "cpu.time":
			stats.CPUTimeNS = value
		case key == "balloon.current":
			stats.MemoryKB = value
		case indexedMetric(key, "block.", ".rd.bytes"):
			stats.DiskReadBytes += value
		case indexedMetric(key, "block.", ".wr.bytes"):
			stats.DiskWriteBytes += value
		case indexedMetric(key, "net.", ".rx.bytes"):
			stats.NetReceiveBytes += value
		case indexedMetric(key, "net.", ".tx.bytes"):
			stats.NetSendBytes += value
		}
	}
	return stats, nil
}

func indexedMetric(key, prefix, suffix string) bool {
	middle, ok := strings.CutPrefix(key, prefix)
	if !ok {
		return false
	}
	middle, ok = strings.CutSuffix(middle, suffix)
	if !ok || middle == "" {
		return false
	}
	for _, r := range middle {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func OpenHubGPU() error {
	path, err := exec.LookPath("ryoku-shell")
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "hub", "open", "gpu")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func RunHub(ctx context.Context, args ...string) ([]byte, error) {
	path, err := exec.LookPath("ryoku-hub")
	if err != nil {
		return nil, errors.New("ryoku-hub is not installed (it owns GPU passthrough)")
	}
	argv := append([]string{"gpu", "vm"}, args...)
	cmd := exec.CommandContext(ctx, path, argv...)
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		detail := strings.TrimSpace(string(exit.Stderr))
		if detail != "" {
			return nil, errors.New(detail)
		}
	}
	return nil, fmt.Errorf("ryoku-hub gpu vm failed: %w", err)
}

func validateCreate(o CreateOptions) error {
	if err := validateName(o.Name); err != nil {
		return err
	}
	if o.OS != "windows" && o.OS != "linux" {
		return errors.New("guest OS must be windows or linux")
	}
	if o.DiskGB < 16 || o.DiskGB > 1024 {
		return errors.New("disk size must be between 16 and 1024 GB")
	}
	if o.RAMMB < 2*1024 || o.RAMMB > 128*1024 || o.RAMMB%1024 != 0 {
		return errors.New("memory must be a whole number of GB between 2 and 128 GB")
	}
	if !filepath.IsAbs(o.ISO) {
		return errors.New("install ISO must be an absolute path")
	}
	info, err := os.Stat(o.ISO)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("install ISO is not a regular file: %s", o.ISO)
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("machine name is required")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("machine name %q must use letters, digits, - or _", name)
	}
	return nil
}

var lookupCommand = exec.LookPath

var commandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
