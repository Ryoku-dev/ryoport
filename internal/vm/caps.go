package vm

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/xdg"
	"golang.org/x/sys/unix"
)

// Caps describes the locally installed quickemu stack.
type Caps struct {
	Quickemu bool   `json:"quickemu"`
	Quickget bool   `json:"quickget"`
	QEMU     bool   `json:"qemu"`
	KVM      bool   `json:"kvm"`
	Spice    bool   `json:"spice"`
	Ready    bool   `json:"ready"`
	Version  string `json:"version"`
}

// Caps reports engine and viewer availability.
func (m *Manager) Caps(ctx context.Context) Caps {
	caps := Caps{
		Quickemu: onPath("quickemu"),
		Quickget: onPath("quickget"),
		QEMU:     onPath("qemu-system-x86_64"),
		KVM:      unix.Access("/dev/kvm", unix.R_OK) == nil,
		Spice:    onPath("spicy") || onPath("remote-viewer"),
	}
	caps.Ready = caps.Quickemu && caps.QEMU && caps.KVM
	if caps.Quickemu {
		command := exec.CommandContext(ctx, "quickemu", "--version")
		if stdout, err := command.StdoutPipe(); err == nil {
			if command.Start() == nil {
				scanner := bufio.NewScanner(stdout)
				if scanner.Scan() {
					caps.Version = scanner.Text()
				}
				_ = command.Wait()
			}
		}
	}
	return caps
}

func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// SetupAdvice describes how the UI can install the VM engine.
type SetupAdvice struct {
	Argv []string
	Hint string
}

// Setup returns a direct Ryoku install command or distro-neutral guidance.
func Setup() SetupAdvice {
	if onPath("ryoku-host") {
		return SetupAdvice{Argv: []string{"ryoku-host", "pkg", "add", "--needed", "spice-gtk", "libisoburn", "quickemu"}}
	}
	return SetupAdvice{Hint: "Install quickemu, spice-gtk and xorriso (libisoburn) with your system package manager."}
}

// Paths matches the ryovm paths compatibility output.
type Paths struct {
	VMs           string `json:"vms"`
	Cache         string `json:"cache"`
	Icons         string `json:"icons"`
	Catalog       string `json:"catalog"`
	Provider      string `json:"provider"`
	IconsProvider string `json:"icons_provider"`
	Source        string `json:"source"`
}

// Paths returns the engine's compatible storage locations.
func (m *Manager) Paths() Paths {
	return Paths{
		VMs:           m.Root,
		Cache:         filepath.Join(xdg.CacheHome(), "ryoku"),
		Icons:         xdg.IconCache(),
		Catalog:       xdg.CatalogCache(),
		Provider:      "quickemu + quickget",
		IconsProvider: "quickemu-icons",
		Source:        "github.com/quickemu-project",
	}
}

func (m *Manager) livePaths(name string) (monitor, agent, pid string, err error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return "", "", "", err
	}
	_, dir, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return "", "", "", err
	}
	return filepath.Join(dir, name+"-monitor.socket"), filepath.Join(dir, name+"-agent.sock"), pid, nil
}

// Stats collects live machine metrics.
func (m *Manager) Stats(ctx context.Context, name string) (qmp.Stats, error) {
	monitor, agent, pid, err := m.livePaths(name)
	if err != nil {
		return qmp.Stats{}, err
	}
	if info, err := os.Stat(agent); err != nil || info.Mode()&os.ModeSocket == 0 {
		agent = ""
	}
	if _, err := os.Stat(pid); err != nil {
		pid = ""
	}
	return qmp.Collect(ctx, monitor, agent, pid)
}

// Power sends a live power action.
func (m *Manager) Power(ctx context.Context, name string, action qmp.PowerAction) error {
	monitor, _, _, err := m.livePaths(name)
	if err != nil {
		return err
	}
	return qmp.Power(ctx, monitor, action)
}

// Balloon changes guest-visible memory in MiB.
func (m *Manager) Balloon(ctx context.Context, name string, mb int) error {
	monitor, _, _, err := m.livePaths(name)
	if err != nil {
		return err
	}
	_, err = qmp.Balloon(ctx, monitor, mb)
	return err
}

// Pin changes live vCPU affinity.
func (m *Manager) Pin(ctx context.Context, name string, on bool) error {
	monitor, _, _, err := m.livePaths(name)
	if err != nil {
		return err
	}
	mode := "off"
	if on {
		mode = "auto"
	}
	_, err = qmp.Pin(ctx, monitor, mode)
	return err
}
