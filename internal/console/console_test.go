package console

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

func TestInstallAdviceUsesHostSeam(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n[ \"$1 $2 $3\" = 'pkg advice freerdp' ] || exit 2\nprintf 'sudo dnf5 install freerdp\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := installAdvice("freerdp"); got != "sudo dnf5 install freerdp" {
		t.Fatalf("installAdvice = %q", got)
	}
}

func TestValidConsole(t *testing.T) {
	good := []Profile{
		{Kind: "serial", Name: "sw", Device: "/dev/ttyUSB0", Baud: 115200},
		{Kind: "telnet", Name: "r", Address: "router.home"},
		{Kind: "rdp", Name: "w", Address: "desk.example", Port: 3390, User: "alice"},
		{Kind: "vnc", Name: "v", Address: "10.0.0.5", Port: 5901},
	}
	for _, profile := range good {
		copy := profile
		if err := validate(&copy); err != nil {
			t.Errorf("valid %+v rejected: %v", profile, err)
		}
	}
	profile := Profile{Kind: "telnet", Name: "r", Address: "h"}
	if err := validate(&profile); err != nil || profile.Port != 23 {
		t.Errorf("telnet default port: got %d, %v", profile.Port, err)
	}
	profile = Profile{Kind: "vnc", Name: "v", Address: "h"}
	if err := validate(&profile); err != nil || profile.Port != 5900 {
		t.Errorf("vnc default port: got %d, %v", profile.Port, err)
	}
	bad := []Profile{
		{Kind: "ssh", Name: "x"},
		{Kind: "rdp", Name: "x", Address: "-gateway:/evil"},
		{Kind: "rdp", Name: "x", Address: "ok; rm -rf /"},
		{Kind: "rdp", Name: "x", Address: "a\nb"},
		{Kind: "serial", Name: "x", Device: "ttyUSB0", Baud: 9600},
		{Kind: "serial", Name: "x", Device: "/dev/ttyUSB 0", Baud: 9600},
		{Kind: "serial", Name: "x", Device: "/dev/ttyUSB0", Baud: 0},
	}
	for _, profile := range bad {
		copy := profile
		if err := validate(&copy); err == nil {
			t.Errorf("invalid %+v accepted", profile)
		}
	}
}

func TestRDPArgvKeepsPasswordOutOfArgv(t *testing.T) {
	profile := Profile{Kind: "rdp", Name: "w", Address: "desk.example", Port: 3390, User: "alice", Domain: "HQ"}
	argv := CommandArgv("xfreerdp3", profile)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "/v:desk.example:3390") || !strings.Contains(joined, "/u:alice") || !strings.Contains(joined, "/d:HQ") || !strings.Contains(joined, "/cert:tofu") {
		t.Fatalf("argv incomplete: %q", joined)
	}
	if strings.Contains(joined, "hunter2") || strings.Contains(joined, "/p:") {
		t.Fatalf("ordinary argv carries a secret: %q", joined)
	}
	profile.Auth = "password"
	if got := strings.Join(CommandArgv("xfreerdp3", profile), " "); got != "xfreerdp3 /args-from:fd:3" {
		t.Fatalf("password RDP argv = %q", got)
	}
	file := rdpArgsFile(profile, "hunter2")
	if !strings.Contains(file, "/p:hunter2") {
		t.Fatalf("args file must carry the password line: %q", file)
	}
	if strings.Contains(file, "\n/p:hunt er2") {
		t.Fatal("password split across lines")
	}
	if !strings.Contains(rdpArgsFile(Profile{Kind: "rdp", Name: "w", Address: "h", Port: 3389}, "p"), "/v:h:3389") {
		t.Error("default port should appear in /v:")
	}
}

func TestSerialArgvUsesRealPicocomFlags(t *testing.T) {
	argv := CommandArgv("picocom", Profile{Kind: "serial", Name: "s", Device: "/dev/ttyUSB0", Baud: 115200})
	want := []string{"picocom", "--baud", "115200", "--flow", "n", "/dev/ttyUSB0"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("serial argv = %v, want %v", argv, want)
	}
	for _, arg := range argv {
		if arg == "--nomap" {
			t.Fatal("picocom has no --nomap option")
		}
	}
}

func TestVNCArgvTargetsExplicitPort(t *testing.T) {
	argv := CommandArgv("vncviewer", Profile{Kind: "vnc", Name: "v", Address: "10.0.0.5", Port: 5901, Fullscreen: true})
	want := []string{"vncviewer", "10.0.0.5::5901", "-FullScreen"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("VNC argv = %v, want %v", argv, want)
	}
}

func TestTelnetArgv(t *testing.T) {
	argv := CommandArgv("telnet", Profile{Kind: "telnet", Name: "r", Address: "router.home", Port: 23})
	if strings.Join(argv, " ") != "telnet router.home 23" {
		t.Fatalf("telnet argv = %v", argv)
	}
}

func TestConsoleProbeSerial(t *testing.T) {
	result := probe(context.Background(), Profile{ID: "x", Kind: "serial", Name: "s", Device: "/dev/null", Baud: 9600})
	if !result.Up {
		t.Fatalf("/dev/null must probe up, got %+v", result)
	}
	result = probe(context.Background(), Profile{ID: "x", Kind: "serial", Name: "s", Device: "/dev/ryokutest-absent", Baud: 9600})
	if result.Up || !strings.Contains(result.Error, "not present") {
		t.Fatalf("absent device must say so, got %+v", result)
	}
	result = probe(context.Background(), Profile{ID: "x", Kind: "serial", Name: "s", Device: "/tmp", Baud: 9600})
	if result.Up || !strings.Contains(result.Error, "device node") {
		t.Fatalf("non-device must say so, got %+v", result)
	}
}

func TestConsoleUpsertRoundTrip(t *testing.T) {
	setConsoleHome(t)
	manager := New()
	if err := manager.Save(Profile{Kind: "rdp", Name: "work", Address: "h", Port: 3389}); err != nil {
		t.Fatal(err)
	}
	profiles, err := manager.List()
	if err != nil || len(profiles) != 1 || profiles[0].ID == "" || profiles[0].Name != "work" {
		t.Fatalf("list after add = %+v, %v", profiles, err)
	}
	profiles[0].Auth = "password"
	if err := writeProfiles(profiles); err != nil {
		t.Fatal(err)
	}
	id := profiles[0].ID
	if err := manager.Save(Profile{ID: id, Kind: "rdp", Name: "work-renamed", Address: "h", Port: 3390}); err != nil {
		t.Fatal(err)
	}
	profiles, _ = manager.List()
	if len(profiles) != 1 || profiles[0].Port != 3390 || profiles[0].Auth != "password" {
		t.Fatalf("edit-save should replace and preserve auth: %+v", profiles)
	}
	if err := manager.Remove(id); err != nil {
		t.Fatal(err)
	}
	profiles, _ = manager.List()
	if len(profiles) != 0 {
		t.Fatalf("remove left %+v", profiles)
	}
}

func TestReadConsolesTolerant(t *testing.T) {
	setConsoleHome(t)
	if err := os.MkdirAll(filepath.Dir(xdg.ConsolesFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdg.ConsolesFile(), []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if profiles := readProfiles(); len(profiles) != 0 {
		t.Fatalf("corrupt file yielded %+v", profiles)
	}
	if err := os.Remove(xdg.ConsolesFile()); err != nil {
		t.Fatal(err)
	}
	if profiles := readProfiles(); profiles == nil || len(profiles) != 0 {
		t.Fatalf("missing file yielded %#v", profiles)
	}
}

func setConsoleHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}
