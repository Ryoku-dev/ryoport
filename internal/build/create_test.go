package build

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/vm"
)

func TestCreateFailedDownloadCleansOnlyNewPartial(t *testing.T) {
	for _, test := range []struct {
		name     string
		existing bool
		disk     bool
		config   bool
		redirect bool
	}{
		{name: "new"},
		{name: "existing", existing: true},
		{name: "new disk", disk: true},
		{name: "new dead config", config: true},
		{name: "existing dead config", existing: true, config: true},
		{name: "other generated name", config: true, redirect: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			testEnvironment(t)
			builder := New(vm.New())
			const requested = "windows-11"
			made := requested
			if test.redirect {
				made += "-other"
			}
			if test.existing {
				if err := os.MkdirAll(builder.VM.Dir(requested), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			body := "#!/bin/sh\nmkdir -p " + made + "\nprintf partial > " + made + "/partial.iso\n"
			if test.disk {
				body += "printf disk > " + made + "/disk.qcow2\n"
			}
			if test.config {
				body += "printf 'guest_os=\"windows\"\niso=\"missing.iso\"\n' > " + made + ".conf\n"
			}
			body += "echo blocked\nexit 1\n"
			writeExecutable(t, "quickget", body)
			if _, err := builder.Create(context.Background(), "windows", "11", "", nil); err == nil {
				t.Fatal("failed download succeeded")
			}
			_, err := os.Stat(builder.VM.Dir(made))
			keep := test.existing || test.disk || test.redirect
			if keep && err != nil {
				t.Fatalf("removed a folder not owned by the failed download: %v", err)
			}
			if !keep && !os.IsNotExist(err) {
				t.Fatalf("new partial folder remains: %v", err)
			}
		})
	}
}

func TestCreateFastConfigFailureCleansNewPartial(t *testing.T) {
	testEnvironment(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("iso"))
	}))
	defer server.Close()
	writeExecutable(t, "quickget", "#!/bin/sh\ncase \"$1\" in\n --url) echo '"+server.URL+"/install.iso';;\n --create-config) mkdir -p \"$2\"; echo blocked; exit 1;;\nesac\n")
	builder := New(vm.New())
	if _, err := builder.Create(context.Background(), "ubuntu", "26.04", "", nil); err == nil {
		t.Fatal("config failure succeeded")
	}
	for _, name := range []string{"ubuntu-26.04", ".staging-ubuntu-26.04"} {
		if _, err := os.Stat(filepath.Join(builder.VM.Root, name)); !os.IsNotExist(err) {
			t.Fatalf("partial %s remains: %v", name, err)
		}
	}
}

func TestCleanPartialPreservesSymlinksAndDisks(t *testing.T) {
	testEnvironment(t)
	builder := New(vm.New())
	if err := os.MkdirAll(builder.VM.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := builder.VM.Dir("linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	builder.cleanPartial("linked", true)
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("removed symbolic link", err)
	}
	dir := builder.VM.Dir("disk")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(dir, "disk.qcow2")); err != nil {
		t.Fatal(err)
	}
	builder.cleanPartial("disk", true)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("removed folder with a disk symlink", err)
	}
}
