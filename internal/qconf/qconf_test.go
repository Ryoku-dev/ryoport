package qconf

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const sample = `#!/usr/bin/quickemu --vm
guest_os="linux"
disk_img="ubuntu-24.04/disk.qcow2"
iso=ubuntu-24.04/ubuntu.iso
# a comment the user wrote
usb_devices=("046d:c52b" "1234:5678")
extra_args="-device foo"
`

func TestGetKeepsUnquotedAndQuotedValues(t *testing.T) {
	c := Parse([]byte(sample))
	if got := c.Get("guest_os"); got != "linux" {
		t.Fatalf("guest_os = %q", got)
	}
	if got := c.Get("iso"); got != "ubuntu-24.04/ubuntu.iso" {
		t.Fatalf("iso = %q", got)
	}
	if got := c.Get("missing"); got != "" || c.Has("missing") {
		t.Fatalf("missing key read as %q", got)
	}
}

func TestListReadsBashArrays(t *testing.T) {
	c := Parse([]byte(sample))
	if got := c.List("usb_devices"); !slices.Equal(got, []string{"046d:c52b", "1234:5678"}) {
		t.Fatalf("usb_devices = %v", got)
	}
	if got := c.List("guest_os"); !slices.Equal(got, []string{"linux"}) {
		t.Fatalf("scalar as list = %v", got)
	}
}

func TestEditsPreserveEverythingElse(t *testing.T) {
	c := Parse([]byte(sample + "guest_os=\"windows\"\n"))
	if err := c.Set("guest_os", "macos"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetList("port_forwards", []string{"8080:80"}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetList("usb_devices", nil); err != nil {
		t.Fatal(err)
	}
	want := `#!/usr/bin/quickemu --vm
guest_os="macos"
disk_img="ubuntu-24.04/disk.qcow2"
iso=ubuntu-24.04/ubuntu.iso
# a comment the user wrote
extra_args="-device foo"
port_forwards=("8080:80")
`
	if got := string(c.Bytes()); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRejectsValuesTheShellWouldExpand(t *testing.T) {
	c := Parse(nil)
	for _, v := range []string{`a"b`, "a$(id)", "a`id`", `a\b`, "a\nb"} {
		if err := c.Set("iso", v); !errors.Is(err, ErrUnsafe) {
			t.Fatalf("Set(%q) = %v, want ErrUnsafe", v, err)
		}
	}
	if err := c.SetList("usb_devices", []string{"a b"}); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("list item with a space = %v", err)
	}
	if err := c.Set("bad-key", "x"); err == nil {
		t.Fatal("bad key accepted")
	}
}

func TestWriteKeepsModeAndReplacesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vm.conf")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("disk_size", "64G"); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	back, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Get("disk_size") != "64G" || back.Get("guest_os") != "linux" {
		t.Fatalf("round trip lost data: %s", back.Bytes())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}
