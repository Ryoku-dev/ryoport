package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/qconf"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

func TestCatalogGroupingAndDefaults(t *testing.T) {
	testEnvironment(t)
	fixture := `[
 {"OS":"ubuntu","Display Name":"Ubuntu","PNG":"u.png","SVG":"u.svg","Release":"22.04","Option":"minimal"},
 {"OS":"ubuntu","Display Name":"Ubuntu","PNG":"u.png","SVG":"u.svg","Release":"24.04","Option":"gnome"},
 {"OS":"ubuntu","Display Name":"Ubuntu","PNG":"u.png","SVG":"u.svg","Release":"24.04","Option":"kde"},
 {"OS":"ubuntu","Display Name":"Ubuntu","PNG":"u.png","SVG":"u.svg","Release":"daily","Option":"default"},
 {"OS":"alpine","Display Name":"Alpine Linux","PNG":"","SVG":"a.svg","Release":"3.23","Option":"standard"}
]`
	fixturePath := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, "quickget", "#!/bin/sh\ncat '"+fixturePath+"'\n")
	builder := New(vm.New())
	systems, err := builder.Catalog(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(systems) != 2 || systems[0].OS != "alpine" || systems[1].OS != "ubuntu" {
		t.Fatalf("grouped systems = %#v", systems)
	}
	ubuntu := systems[1]
	if got := strings.Join(ubuntu.Releases, ","); got != "22.04,24.04,daily" {
		t.Fatalf("releases = %q", got)
	}
	if got := strings.Join(ubuntu.Editions["24.04"], ","); got != "gnome,kde" {
		t.Fatalf("editions = %q", got)
	}
	if DefaultRelease(ubuntu) != "24.04" {
		t.Fatalf("default release = %q", DefaultRelease(ubuntu))
	}
	if DefaultEdition(ubuntu, "24.04") != "gnome" {
		t.Fatalf("default edition = %q", DefaultEdition(ubuntu, "24.04"))
	}
}

func TestImportWritesCompatibleConfig(t *testing.T) {
	testEnvironment(t)
	manager := vm.New()
	builder := New(manager)
	iso := filepath.Join(t.TempDir(), "windows.iso")
	if err := os.WriteFile(iso, []byte("iso"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := builder.Import("win11", iso, "windows", "windows"); err != nil {
		t.Fatal(err)
	}
	conf, err := qconf.Read(manager.ConfPath("win11"))
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"guest_os": "windows", "disk_img": "win11/disk.qcow2", "iso": iso,
		"disk_size": "64G", "ryovm_os": "windows", "tpm": "on",
		"secureboot": "off", "fixed_iso": filepath.Join(xdg.ImagesDir(), "virtio-win.iso"),
	}
	for key, want := range checks {
		if got := conf.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestInstantWritesConfigAndSeed(t *testing.T) {
	testEnvironment(t)
	capture := filepath.Join(t.TempDir(), "seed-input")
	t.Setenv("RYOPORT_SEED_CAPTURE", capture)
	writeBuildTools(t)
	if err := os.MkdirAll(xdg.ImagesDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg.ImagesDir(), "debian-cloud.qcow2"), []byte("base"), 0o600); err != nil {
		t.Fatal(err)
	}
	builder := New(vm.New())
	name, err := builder.Instant(context.Background(), InstantOptions{
		OS: "debian", Name: "devbox", Disposable: true,
		Tools: []string{"git", "spice"}, Pkgs: []string{"ripgrep"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if name != "devbox" {
		t.Fatalf("name = %q", name)
	}
	conf, err := qconf.Read(builder.VM.ConfPath(name))
	if err != nil {
		t.Fatal(err)
	}
	if conf.Get("ryovm_burn") != "1" || conf.Get("ryovm_ssh_user") != "ryoku" || conf.Get("ryovm_tools") != "git,spice" {
		t.Fatalf("instant config = %s", conf.Bytes())
	}
	seed, err := os.ReadFile(filepath.Join(capture, "user-data"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(seed)
	for _, want := range []string{"name: ryoku", "plain_text_passwd: ryoku", "ssh_authorized_keys:", "path: /usr/local/bin/clip", "  - git", "  - spice-vdagent", "  - ripgrep", "  - bash-completion"} {
		if !strings.Contains(text, want) {
			t.Errorf("seed missing %q\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(builder.VM.Dir(name), "seed.iso")); err != nil {
		t.Fatal(err)
	}
}

func TestVirtIOISOValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtio.iso")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(virtioMinimumSize); err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("CD001"), 0x8001); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if !validISO(path) {
		t.Fatal("valid ISO rejected")
	}
	if err := os.WriteFile(path, []byte("<html>blocked</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if validISO(path) {
		t.Fatal("HTML stub accepted")
	}
}

func TestTemplateMetadataRoundTrip(t *testing.T) {
	testEnvironment(t)
	writeExecutable(t, "qemu-img", `#!/bin/sh
if [ "$1" = convert ]; then cp "$4" "$5"; exit; fi
exit 1
`)
	manager := vm.New()
	if err := os.MkdirAll(manager.Dir("source"), 0o700); err != nil {
		t.Fatal(err)
	}
	conf := qconf.Parse(nil)
	_ = conf.Set("disk_img", "source/disk.qcow2")
	_ = conf.Set("ryovm_os", "debian")
	_ = conf.Set("ryovm_tools", "git,go")
	if err := conf.Write(manager.ConfPath("source")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manager.Dir("source"), "disk.qcow2"), []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	builder := New(manager)
	if err := builder.MakeTemplate(context.Background(), "source", "golden"); err != nil {
		t.Fatal(err)
	}
	templates, err := Templates()
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 {
		t.Fatalf("templates = %#v", templates)
	}
	got := templates[0]
	if got.Name != "golden" || got.OS != "debian" || got.Tools != "git,go" || got.Base != filepath.Join(xdg.TemplatesDir(), "golden.qcow2") {
		t.Fatalf("template = %#v", got)
	}
	data, err := os.ReadFile(filepath.Join(xdg.TemplatesDir(), "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["name"]; present {
		t.Fatal("metadata file must not contain derived name")
	}
}

func testEnvironment(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("RYOVM_HOME", filepath.Join(root, "vms"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func writeExecutable(t *testing.T, name, body string) string {
	t.Helper()
	bin := filepath.SplitList(os.Getenv("PATH"))[0]
	path := filepath.Join(bin, name)
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeBuildTools(t *testing.T) {
	t.Helper()
	writeExecutable(t, "qemu-img", `#!/bin/sh
case "$1" in
 create) : > "$8" ;;
 snapshot) exit 0 ;;
 *) exit 1 ;;
esac
`)
	writeExecutable(t, "ssh-keygen", `#!/bin/sh
while [ "$#" -gt 0 ]; do
 if [ "$1" = -f ]; then shift; key="$1"; fi
 shift
done
mkdir -p "$(dirname "$key")"
printf private > "$key"
printf 'ssh-ed25519 AAAATEST ryovm-burn\n' > "$key.pub"
`)
	writeExecutable(t, "genisoimage", `#!/bin/sh
want_out=
prev=
last=
for arg; do
 if [ "$want_out" = 1 ]; then out="$arg"; want_out=; fi
 if [ "$arg" = -output ]; then want_out=1; fi
 prev="$last"
 last="$arg"
done
mkdir -p "$RYOPORT_SEED_CAPTURE"
cp "$prev" "$RYOPORT_SEED_CAPTURE/user-data"
cp "$last" "$RYOPORT_SEED_CAPTURE/meta-data"
printf cidata > "$out"
`)
	writeExecutable(t, "quickemu", `#!/bin/sh
while [ "$#" -gt 0 ]; do
 if [ "$1" = --vm ]; then shift; conf="$1"; fi
 shift
done
name="${conf%.conf}"
mkdir -p "$name"
echo $$ > "$name/$name.pid"
sleep 2
`)
}
