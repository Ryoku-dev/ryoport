// Package xdg resolves every path Ryoport touches. The locations are the ones
// the old Ryoport used, so existing machines, servers and keys keep working.
package xdg

import (
	"os"
	"path/filepath"
)

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/"
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" && filepath.IsAbs(v) {
		return v
	}
	return fallback
}

func DataHome() string   { return env("XDG_DATA_HOME", filepath.Join(home(), ".local", "share")) }
func ConfigHome() string { return env("XDG_CONFIG_HOME", filepath.Join(home(), ".config")) }
func CacheHome() string  { return env("XDG_CACHE_HOME", filepath.Join(home(), ".cache")) }
func StateHome() string  { return env("XDG_STATE_HOME", filepath.Join(home(), ".local", "state")) }

// RuntimeDir falls back to the cache dir when the session has no runtime dir.
func RuntimeDir() string { return env("XDG_RUNTIME_DIR", CacheHome()) }

func Home() string { return home() }

// VMRoot holds quickemu machines: <name>.conf next to a <name>/ directory.
func VMRoot() string {
	if value := os.Getenv("RYOVM_HOME"); value != "" {
		if absolute, err := filepath.Abs(value); err == nil {
			return absolute
		}
	}
	return filepath.Join(DataHome(), "ryoku", "vms")
}

// ImagesDir holds shared cloud bases, the VirtIO CD and templates.
func ImagesDir() string { return filepath.Join(VMRoot(), ".images") }

func TemplatesDir() string { return filepath.Join(ImagesDir(), "templates") }

// BurnKey is the private key instant machines trust.
func BurnKey() string { return filepath.Join(DataHome(), "ryoku", "keys", "ryovm") }

func CatalogCache() string { return filepath.Join(CacheHome(), "ryoku", "ryovm-catalog.json") }

func IconCache() string { return filepath.Join(CacheHome(), "ryoku", "ryovm-icons") }

// PassthroughRoot is owned by the Hub's GPU VM backend; Ryoport only reads it.
func PassthroughRoot() string { return env("RYOKU_VM_ROOT", filepath.Join(DataHome(), "ryoku", "vm")) }

// ConfigDir holds remotes.json, consoles.json and settings.json.
func ConfigDir() string { return filepath.Join(ConfigHome(), "ryoku", "ryoport") }

func RemotesFile() string  { return filepath.Join(ConfigDir(), "remotes.json") }
func ConsolesFile() string { return filepath.Join(ConfigDir(), "consoles.json") }
func SettingsFile() string { return filepath.Join(ConfigDir(), "settings.json") }

// LegacyDefaults is the old app's new-machine defaults, imported once.
func LegacyDefaults() string { return filepath.Join(ConfigHome(), "ryoku", "ryovm.json") }

// StateDir holds tunnels.json, the askpass helper and the activity log.
func StateDir() string { return filepath.Join(StateHome(), "ryoku", "ryoport") }

func TunnelsFile() string { return filepath.Join(StateDir(), "tunnels.json") }
func AskpassFile() string { return filepath.Join(StateDir(), "askpass") }
func EventsFile() string  { return filepath.Join(StateDir(), "events.jsonl") }

// UpdateDir stages downloads for the self-updater.
func UpdateDir() string { return filepath.Join(CacheHome(), "ryoport", "update") }

func SSHDir() string         { return filepath.Join(home(), ".ssh") }
func SSHConfig() string      { return filepath.Join(SSHDir(), "config") }
func SSHManaged() string     { return filepath.Join(SSHDir(), "config.d", "ryoport") }
func SSHControlPath() string { return filepath.Join(SSHDir(), "ryoport-%r@%h:%p") }

func Socket() string { return filepath.Join(RuntimeDir(), "ryoport.sock") }

// RyokuPalette is the live palette the Ryoku shell writes on wallpaper change.
func RyokuPalette() string { return filepath.Join(CacheHome(), "ryoku", "colors.json") }

func RyokuTheme() string { return filepath.Join(ConfigHome(), "ryoku", "theme.json") }

func RyokuShellSettings() string { return filepath.Join(ConfigHome(), "ryoku", "shell.json") }

// EnsureDir makes dir with mode 0700 when it is missing.
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }
