// Package config loads and saves Ryoport settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

type Appearance struct {
	Mode         string `json:"mode"`
	Density      string `json:"density"`
	ReduceMotion bool   `json:"reduceMotion"`
}

type MachineDefaults struct {
	Cores     int      `json:"cores"`
	RAMGB     int      `json:"ramGb"`
	DiskGB    int      `json:"diskGb"`
	Display   string   `json:"display"`
	Tools     []string `json:"tools"`
	ExtraPkgs string   `json:"extraPkgs"`
}

type Updates struct {
	Check     bool      `json:"check"`
	LastCheck time.Time `json:"lastCheck,omitempty"`
	Skipped   string    `json:"skipped,omitempty"`
}

type Settings struct {
	Appearance Appearance      `json:"appearance"`
	Defaults   MachineDefaults `json:"defaults"`
	Pinned     []string        `json:"pinned"`
	Grouping   string          `json:"grouping"`
	Updates    Updates         `json:"updates"`
}

func Defaults() Settings {
	return Settings{
		Appearance: Appearance{Mode: "follow", Density: "comfortable"},
		Defaults: MachineDefaults{
			Cores: 4, RAMGB: 8, DiskGB: 64, Display: "window",
			Tools: []string{"git", "build", "cli", "spice"},
		},
		Pinned:   []string{},
		Grouping: "kind",
		Updates:  Updates{Check: true},
	}
}

func Load() (Settings, error) {
	settings := Defaults()
	data, err := os.ReadFile(xdg.SettingsFile())
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return Settings{}, fmt.Errorf("read settings: %w", err)
		}
		normalize(&settings)
		return settings, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Settings{}, err
	}
	legacy, imported, err := loadLegacy(settings)
	if err != nil {
		return Settings{}, err
	}
	if imported {
		if err := Save(legacy); err != nil {
			return Settings{}, fmt.Errorf("save imported settings: %w", err)
		}
	}
	return legacy, nil
}

func Save(settings Settings) error {
	normalize(&settings)
	if err := xdg.EnsureDir(xdg.ConfigDir()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWrite(xdg.SettingsFile(), data, 0o600)
}

func loadLegacy(settings Settings) (Settings, bool, error) {
	var legacy struct {
		DefaultCores   *int    `json:"defaultCores"`
		DefaultRAM     *int    `json:"defaultRam"`
		DefaultDisk    *int    `json:"defaultDisk"`
		DefaultDisplay *string `json:"defaultDisplay"`
		Tools          *string `json:"tools"`
		ExtraPkgs      *string `json:"extraPkgs"`
	}
	data, err := os.ReadFile(xdg.LegacyDefaults())
	if errors.Is(err, os.ErrNotExist) {
		return settings, false, nil
	}
	if err != nil {
		return Settings{}, false, err
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Settings{}, false, fmt.Errorf("read legacy defaults: %w", err)
	}
	if legacy.DefaultCores != nil && *legacy.DefaultCores > 0 {
		settings.Defaults.Cores = *legacy.DefaultCores
	}
	if legacy.DefaultRAM != nil && *legacy.DefaultRAM > 0 {
		settings.Defaults.RAMGB = *legacy.DefaultRAM
	}
	if legacy.DefaultDisk != nil && *legacy.DefaultDisk > 0 {
		settings.Defaults.DiskGB = *legacy.DefaultDisk
	}
	if legacy.DefaultDisplay != nil && *legacy.DefaultDisplay != "" {
		settings.Defaults.Display = *legacy.DefaultDisplay
	}
	if legacy.Tools != nil {
		settings.Defaults.Tools = splitTools(*legacy.Tools)
	}
	if legacy.ExtraPkgs != nil {
		settings.Defaults.ExtraPkgs = *legacy.ExtraPkgs
	}
	return settings, true, nil
}

func splitTools(value string) []string {
	var tools []string
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			tools = append(tools, part)
		}
	}
	return tools
}

func normalize(settings *Settings) {
	if settings.Pinned == nil {
		settings.Pinned = []string{}
	}
	if settings.Defaults.Tools == nil {
		settings.Defaults.Tools = []string{}
	}
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
