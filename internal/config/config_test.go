package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

func TestLoadImportsLegacyDefaultsOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	if err := os.MkdirAll(filepath.Dir(xdg.LegacyDefaults()), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"defaultCores":6,"defaultRam":12,"defaultDisk":96,"defaultDisplay":"spice","tools":"git, cli,custom","extraPkgs":"ripgrep fd"}`
	if err := os.WriteFile(xdg.LegacyDefaults(), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Defaults.Cores != 6 || got.Defaults.RAMGB != 12 || got.Defaults.DiskGB != 96 || got.Defaults.Display != "spice" {
		t.Fatalf("legacy defaults not imported: %+v", got.Defaults)
	}
	if !reflect.DeepEqual(got.Defaults.Tools, []string{"git", "cli", "custom"}) || got.Defaults.ExtraPkgs != "ripgrep fd" {
		t.Fatalf("legacy tools not imported: %+v", got.Defaults)
	}
	if _, err := os.Stat(xdg.SettingsFile()); err != nil {
		t.Fatalf("import did not write settings: %v", err)
	}
	if err := os.WriteFile(xdg.LegacyDefaults(), []byte(`{"defaultCores":20}`), 0o600); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Defaults.Cores != 6 {
		t.Fatalf("legacy file was imported more than once: %d", again.Defaults.Cores)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	want := Defaults()
	want.Appearance = Appearance{Mode: "dark", Density: "compact", ReduceMotion: true}
	want.Pinned = []string{"machine:lab", "passthrough:win11", "console:switch"}
	want.Grouping = "group"
	want.Updates = Updates{Check: false, LastCheck: time.Date(2026, 11, 8, 1, 2, 3, 0, time.UTC), Skipped: "1.2.3"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", got, want)
	}
	info, err := os.Stat(xdg.SettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode = %o", info.Mode().Perm())
	}
}

func TestLoadDropsUnknownFieldsOnSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	if err := os.MkdirAll(xdg.ConfigDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdg.SettingsFile(), []byte(`{"unknown":true,"appearance":{"mode":"light"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Appearance.Mode != "light" {
		t.Fatalf("mode = %q", settings.Appearance.Mode)
	}
	if err := Save(settings); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(xdg.SettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"unknown"`)) {
		t.Fatal("unknown field was retained")
	}
}
