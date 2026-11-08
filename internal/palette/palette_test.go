package palette

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func paletteEnvironment(t *testing.T) (config, cache string) {
	t.Helper()
	root := t.TempDir()
	config = filepath.Join(root, "config")
	cache = filepath.Join(root, "cache")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	previousOSRelease := osReleasePath
	osReleasePath = filepath.Join(root, "missing-os-release")
	t.Cleanup(func() { osReleasePath = previousOSRelease })
	return config, cache
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadResolutionOrder(t *testing.T) {
	config, cache := paletteEnvironment(t)
	colors := filepath.Join(cache, "ryoku", "colors.json")
	theme := filepath.Join(config, "ryoku", "theme.json")
	shell := filepath.Join(config, "ryoku", "shell.json")

	if got := Load(Follow); got.Source != "signature" || !got.Dark || len(got.Roles) != 0 {
		t.Fatalf("empty desktop = %#v", got)
	}

	writeJSON(t, colors, `{"surface":"#f2f2f2","onSurface":"#121212","primary":"#334455"}`)
	wall := Load(Follow)
	if wall.Source != "wallpaper" || wall.Dark {
		t.Fatalf("wallpaper = %#v", wall)
	}

	writeJSON(t, shell, `{"themePalette":{"surface":"#101112","primary":"#aabbcc"}}`)
	scheme := Load(Follow)
	if scheme.Source != "scheme" || !scheme.Dark {
		t.Fatalf("scheme = %#v", scheme)
	}
	if got := scheme.Roles["primary"]; got != (color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}) {
		t.Errorf("named primary = %#v", got)
	}
	if got := scheme.Roles["onSurface"]; got != wall.Roles["onSurface"] {
		t.Errorf("partial scheme did not fall through: %#v", got)
	}

	writeJSON(t, theme, `{"followWallpaper":false}`)
	writeJSON(t, shell, `{}`)
	locked := Load(Follow)
	if locked.Source != "signature" || len(locked.Roles) != 0 || !locked.Dark {
		t.Fatalf("locked = %#v", locked)
	}

	if got := Load(Dark); got.Source != "wallpaper" || !got.Dark || !got.Forced ||
		got.Roles["primary"] != wall.Roles["primary"] {
		t.Fatalf("dark mode = %#v", got)
	}
	if got := Load(Light); got.Source != "wallpaper" || got.Dark || !got.Forced ||
		got.Roles["primary"] != wall.Roles["primary"] {
		t.Fatalf("light mode = %#v", got)
	}
}

func TestLoadMalformedJSONFallsBack(t *testing.T) {
	config, cache := paletteEnvironment(t)
	writeJSON(t, filepath.Join(config, "ryoku", "shell.json"), `{"themePalette":`)
	writeJSON(t, filepath.Join(config, "ryoku", "theme.json"), `{"followWallpaper":`)
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), `{"surface":`)
	got := Load(Follow)
	if got.Source != "signature" || len(got.Roles) != 0 || !got.Dark {
		t.Fatalf("malformed files = %#v", got)
	}
}

func TestLoadDetectsLightSurface(t *testing.T) {
	_, cache := paletteEnvironment(t)
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), `{"surface":"#818181"}`)
	if got := Load(Follow); got.Dark {
		t.Fatalf("#818181 classified dark: %#v", got)
	}
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), `{"surface":"#7f7f7f"}`)
	if got := Load(Follow); !got.Dark {
		t.Fatalf("#7f7f7f classified light: %#v", got)
	}
}

func TestLoadForcedModeKeepsRyokuAccents(t *testing.T) {
	_, cache := paletteEnvironment(t)
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), `{
		"surface":"#f5f5f5",
		"primary":"#6750a4",
		"onPrimary":"#ffffff",
		"secondaryContainer":"#e8def8",
		"onSecondaryContainer":"#1d192b",
		"error":"#ba1a1a"
	}`)
	for _, test := range []struct {
		mode Mode
		dark bool
	}{
		{Dark, true},
		{Light, false},
	} {
		got := Load(test.mode)
		if got.Source != "wallpaper" || got.Dark != test.dark || !got.Forced {
			t.Fatalf("forced mode %v = %#v", test.mode, got)
		}
		for _, role := range []string{"primary", "onPrimary", "secondaryContainer", "onSecondaryContainer", "error"} {
			if _, ok := got.Roles[role]; !ok {
				t.Errorf("forced mode %v dropped %s", test.mode, role)
			}
		}
	}
}

func TestEditionColorFromOSRelease(t *testing.T) {
	tests := []struct {
		name string
		body string
		want color.NRGBA
	}{
		{"id", "NAME=Arch Linux\nID=arch\nID_LIKE=debian\n", color.NRGBA{R: 0x17, G: 0x93, B: 0xd1, A: 0xff}},
		{"id like", "ID=custom\nID_LIKE=\"debian ubuntu\"\n", color.NRGBA{R: 0xd7, G: 0x0a, B: 0x53, A: 0xff}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "os-release")
			writeJSON(t, path, test.body)
			got, ok := editionColor(path)
			if !ok || got != test.want {
				t.Fatalf("edition = %#v, %v; want %#v", got, ok, test.want)
			}
		})
	}
}

func TestLoadUsesEditionAccentWithoutRyokuPalette(t *testing.T) {
	paletteEnvironment(t)
	osReleasePath = filepath.Join(t.TempDir(), "os-release")
	writeJSON(t, osReleasePath, "ID=fedora\n")
	got := Load(Follow)
	want := color.NRGBA{R: 0x51, G: 0xa2, B: 0xda, A: 0xff}
	if got.Source != "signature" || got.Roles["primary"] != want {
		t.Fatalf("edition palette = %#v, want primary %#v", got, want)
	}
}

func TestWatchAtomicRenameAndIgnoresUnrelatedFiles(t *testing.T) {
	_, cache := paletteEnvironment(t)
	dir := filepath.Join(cache, "ryoku")
	colors := filepath.Join(dir, "colors.json")
	writeJSON(t, colors, `{"surface":"#101010"}`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan Palette, 2)
	errors := make(chan error, 1)
	ready := make(chan struct{})
	var readyOnce sync.Once
	mode := func() Mode {
		readyOnce.Do(func() { close(ready) })
		return Follow
	}
	go func() { errors <- Watch(ctx, mode, func(p Palette) { changes <- p }) }()
	select {
	case <-ready:
	case err := <-errors:
		t.Fatalf("watch did not start: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not start")
	}

	writeJSON(t, filepath.Join(dir, "unrelated.json"), `{"surface":"#ffffff"}`)
	select {
	case got := <-changes:
		t.Fatalf("unrelated file triggered %#v", got)
	case err := <-errors:
		t.Fatalf("watch stopped early: %v", err)
	case <-time.After(350 * time.Millisecond):
	}

	temporary := filepath.Join(dir, ".colors.json.new")
	writeJSON(t, temporary, `{"surface":"#f0f0f0","primary":"#123456"}`)
	if err := os.Rename(temporary, colors); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-changes:
		if got.Dark || got.Source != "wallpaper" || got.Roles["primary"] != (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}) {
			t.Fatalf("atomic rename = %#v", got)
		}
	case err := <-errors:
		t.Fatalf("watch stopped early: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not report atomic rename")
	}

	cancel()
	select {
	case err := <-errors:
		if err != nil {
			t.Fatalf("watch cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not stop after cancellation")
	}
}
