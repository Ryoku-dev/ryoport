package vm

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestListAndGetFixtures(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "qemu-img"), `#!/bin/sh
if [ "$1" = info ]; then
cat <<'JSON'
{"snapshots":[{"name":"first","date-sec":10},{"name":"sealed","date-sec":20}]}
JSON
fi
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("USER", "fixture-user")
	writeFile(t, filepath.Join(root, "alpha.conf"), `guest_os="linux"
disk_img="alpha/disk.qcow2"
ryovm_os="debian"
cpu_cores="4"
ram="8G"
disk_size="64G"
tpm="on"
secureboot="on"
`)
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "alpha", "disk.qcow2"), "disk")

	manager := &Manager{Root: root}
	machines, err := manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 1 {
		t.Fatalf("List() returned %d machines", len(machines))
	}
	machine := machines[0]
	if machine.Name != "alpha" || machine.Guest != "linux" || machine.OS != "debian" || machine.Display != "gtk" {
		t.Fatalf("unexpected machine: %+v", machine)
	}
	if machine.Cores != "4" || machine.RAM != "8G" || machine.Disk != "64G" || !machine.Installed || !machine.TPM || !machine.SecureBoot || !machine.UEFI {
		t.Fatalf("unexpected machine facts: %+v", machine)
	}

	detail, err := manager.Get(context.Background(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if detail.SSHUser != "fixture-user" || detail.Conf != filepath.Join(root, "alpha.conf") || detail.Dir != filepath.Join(root, "alpha") {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	wantSnapshots := []Snapshot{
		{Name: "sealed", Date: "1970-01-01T00:00:20Z"},
		{Name: "first", Date: "1970-01-01T00:00:10Z"},
	}
	if !reflect.DeepEqual(detail.Snapshots, wantSnapshots) || !detail.Sealed {
		t.Fatalf("snapshots = %+v, sealed = %v", detail.Snapshots, detail.Sealed)
	}
}

func TestConfigUSBAndPortForwardRoundTrip(t *testing.T) {
	root := t.TempDir()
	manager := &Manager{Root: root}
	confPath := filepath.Join(root, "box.conf")
	writeFile(t, confPath, `guest_os="linux"
disk_img="box/disk.qcow2"
usb_devices=("1111:2222")
port_forwards=("2022:22" "8080:80")
`)

	if err := manager.SetConfig("box", "ram", "8G"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetUSB("box", "abcd:1234", true); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetUSB("box", "1111:2222", false); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddPortForward("box", "2022:2222"); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemovePortForward("box", "8080:80"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, line := range []string{
		`ram="8G"`,
		`usb_devices=("abcd:1234")`,
		`port_forwards=("2022:2222")`,
	} {
		if !strings.Contains(text, line+"\n") {
			t.Fatalf("config missing %q:\n%s", line, text)
		}
	}
	forwards, err := manager.PortForwards("box")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forwards, []PortForward{{Host: "2022", Guest: "2222"}}) {
		t.Fatalf("PortForwards() = %+v", forwards)
	}
}

func TestRenameMovesFilesAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	manager := &Manager{Root: root}
	writeFile(t, filepath.Join(root, "old.conf"), `disk_img="old/disk.qcow2"
iso="old/install.iso"
fixed_iso="/shared/driver.iso"
`)
	oldDir := filepath.Join(root, "old")
	if err := os.Mkdir(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(oldDir, "disk.qcow2"), "disk")
	writeFile(t, filepath.Join(oldDir, "old.launch.log"), "log")
	writeFile(t, filepath.Join(oldDir, "old-monitor.socket"), "socket")

	if err := manager.Rename("old", "new"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "new.conf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `disk_img="new/disk.qcow2"`) || !strings.Contains(text, `iso="new/install.iso"`) || !strings.Contains(text, `fixed_iso="/shared/driver.iso"`) {
		t.Fatalf("renamed config =\n%s", text)
	}
	for _, path := range []string{
		filepath.Join(root, "new", "new.launch.log"),
		filepath.Join(root, "new", "new-monitor.socket"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing renamed runtime file %s: %v", path, err)
		}
	}
}

func TestSSHCommandOrdinaryAndBurn(t *testing.T) {
	root := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("USER", "host-user")
	manager := &Manager{Root: root}
	writeFile(t, filepath.Join(root, "box.conf"), `disk_img="box/disk.qcow2"
ryovm_ssh_user="guest"
`)
	boxDir := filepath.Join(root, "box")
	if err := os.Mkdir(boxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(boxDir, "box.pid"), processID())
	writeFile(t, filepath.Join(boxDir, "box.ports"), "ssh,2222\n")
	burnKey := filepath.Join(dataHome, "ryoku", "keys", "ryovm")
	if err := os.MkdirAll(filepath.Dir(burnKey), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, burnKey, "key")

	ordinary, err := manager.SSHCommand("box")
	if err != nil {
		t.Fatal(err)
	}
	wantOrdinary := []string{
		"env", "TERM=xterm-256color", "ssh",
		"-o", "UserKnownHostsFile=" + filepath.Join(boxDir, "box.known_hosts"),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=8",
		"-i", burnKey,
		"-p", "2222", "guest@localhost",
	}
	if !reflect.DeepEqual(ordinary, wantOrdinary) {
		t.Fatalf("ordinary argv = %#v", ordinary)
	}

	if err := manager.SetConfig("box", "ryovm_burn", "1"); err == nil {
		t.Fatal("SetConfig should reject a running machine")
	}
	writeFile(t, filepath.Join(root, "burn.conf"), `disk_img="burn/disk.qcow2"
ryovm_ssh_user="ryoku"
ryovm_burn="1"
`)
	burnDir := filepath.Join(root, "burn")
	if err := os.Mkdir(burnDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(burnDir, "burn.pid"), processID())
	writeFile(t, filepath.Join(burnDir, "burn.ports"), "ssh,2223\n")
	burn, err := manager.SSHCommand("burn")
	if err != nil {
		t.Fatal(err)
	}
	wantBurn := []string{
		"env", "TERM=xterm-256color", "ssh",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=8",
		"-i", burnKey,
		"-p", "2223", "ryoku@localhost",
	}
	if !reflect.DeepEqual(burn, wantBurn) {
		t.Fatalf("burn argv = %#v", burn)
	}
}

func TestValidation(t *testing.T) {
	for _, name := range []string{"alpha", "ubuntu-24.04", "a_b"} {
		if !ValidName(name) {
			t.Errorf("ValidName(%q) = false", name)
		}
	}
	for _, name := range []string{"", ".", "..", "has space", "a/b", "-flag"} {
		if ValidName(name) {
			t.Errorf("ValidName(%q) = true", name)
		}
	}
	manager := &Manager{Root: t.TempDir()}
	if err := manager.Resize(context.Background(), "missing", "64GiB"); err == nil || err.Error() != "size must look like 64G or 4096M" {
		t.Fatalf("bad size error = %v", err)
	}
	if err := manager.AddPortForward("missing", "0:22"); err == nil || err.Error() != "port out of range in '0:22'" {
		t.Fatalf("bad port error = %v", err)
	}
	if err := manager.AddPortForward("missing", "host:22"); err == nil || err.Error() != "bad forward 'host:22' (want hostport:guestport)" {
		t.Fatalf("bad forward error = %v", err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func processID() string {
	return strconv.Itoa(os.Getpid())
}
