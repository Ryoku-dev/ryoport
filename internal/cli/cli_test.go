package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cliEnvironment(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{home, bin, filepath.Join(root, "data"), filepath.Join(root, "config"), filepath.Join(root, "cache"), filepath.Join(root, "state"), filepath.Join(root, "run"), filepath.Join(root, "vms")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USER", "tester")
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("RYOVM_HOME", filepath.Join(root, "vms"))
	t.Setenv("PATH", bin)
	for _, name := range []string{"qemu-img", "quickget", "ssh", "secret-tool", "virsh", "ryoku-hub"} {
		body := "#!/bin/sh\nexit 0\n"
		switch name {
		case "ryoku-hub":
			body = "#!/bin/sh\nprintf '%s\\n' \"$*\"\n"
		case "secret-tool":
			body = "#!/bin/sh\n[ \"$1\" = lookup ] && printf 's3cr3t\\n'\nexit 0\n"
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root, bin
}

func runCLI(t *testing.T, argv ...string) (int, bool, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code, gui := Main(argv, &stdout, &stderr)
	return code, gui, stdout.String(), stderr.String()
}

func TestRyoportDispatch(t *testing.T) {
	cliEnvironment(t)
	tests := []struct {
		name       string
		argv       []string
		code       int
		gui        bool
		stdoutPart string
		stderrPart string
	}{
		{name: "window", argv: []string{"ryoport"}, gui: true},
		{name: "window flags", argv: []string{"ryoport", "--page", "activity", "--select", "server:ally"}, gui: true},
		{name: "version", argv: []string{"ryoport", "version"}, stdoutPart: "1.0.0\n"},
		{name: "wait dead pid", argv: []string{"ryoport", "--wait-pid", "2147483647"}, gui: true},
		{name: "list passthrough", argv: []string{"ryoport", "list"}, stdoutPart: "gpu vm list\n"},
		{name: "machine shortcut", argv: []string{"ryoport", "win11"}, stdoutPart: "gpu vm start --name win11\n"},
		{name: "unknown flag", argv: []string{"ryoport", "--wat"}, code: 2, stderrPart: "usage: ryoport"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, gui, stdout, stderr := runCLI(t, test.argv...)
			if code != test.code || gui != test.gui {
				t.Fatalf("Main = (%d, %v), want (%d, %v); stderr=%q", code, gui, test.code, test.gui, stderr)
			}
			if test.stdoutPart != "" && !strings.Contains(stdout, test.stdoutPart) {
				t.Fatalf("stdout %q does not contain %q", stdout, test.stdoutPart)
			}
			if test.stderrPart != "" && !strings.Contains(stderr, test.stderrPart) {
				t.Fatalf("stderr %q does not contain %q", stderr, test.stderrPart)
			}
		})
	}
}

func TestParseWindow(t *testing.T) {
	selected, page := ParseWindow([]string{"--select", "machine:debian", "--page", "settings"})
	if selected != "machine:debian" || page != "settings" {
		t.Fatalf("ParseWindow = %q, %q", selected, page)
	}
	selected, page = ParseWindow(nil)
	if selected != "" || page != "fleet" {
		t.Fatalf("ParseWindow defaults = %q, %q", selected, page)
	}
	selected, page = ParseWindow([]string{"--wait-pid", "2147483647"})
	if selected != "" || page != "fleet" {
		t.Fatalf("ParseWindow wait defaults = %q, %q", selected, page)
	}
}

func TestRYOVMOutputShapes(t *testing.T) {
	root, _ := cliEnvironment(t)
	conf := "guest_os=\"linux\"\ndisk_img=\"box/disk.qcow2\"\n"
	if err := os.WriteFile(filepath.Join(root, "vms", "box.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		argv    []string
		code    int
		shape   string
		literal string
	}{
		{name: "list", argv: []string{"ryovm", "list"}, shape: "array"},
		{name: "get missing", argv: []string{"ryovm", "get", "missing"}, code: 1, literal: "no VM named 'missing'"},
		{name: "cloud catalogue", argv: []string{"ryovm", "cloud-catalog"}, shape: "array"},
		{name: "paths", argv: []string{"ryovm", "paths"}, shape: "object"},
		{name: "templates", argv: []string{"ryovm", "templates"}, shape: "array"},
		{name: "config", argv: []string{"ryovm", "config", "box", "ram", "4G"}, literal: "set ram=4G on box\n"},
		{name: "bad verb", argv: []string{"ryovm", "wat"}, code: 2, literal: "usage: ryovm"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, gui, stdout, stderr := runCLI(t, test.argv...)
			if code != test.code || gui {
				t.Fatalf("Main = (%d, %v), want (%d, false), stderr=%q", code, gui, test.code, stderr)
			}
			combined := stdout + stderr
			if test.literal != "" && !strings.Contains(combined, test.literal) {
				t.Fatalf("output %q does not contain %q", combined, test.literal)
			}
			if test.shape != "" {
				var value any
				if err := json.Unmarshal([]byte(stdout), &value); err != nil {
					t.Fatalf("invalid JSON %q: %v", stdout, err)
				}
				switch test.shape {
				case "array":
					if _, ok := value.([]any); !ok {
						t.Fatalf("JSON shape = %T, want array", value)
					}
				case "object":
					if _, ok := value.(map[string]any); !ok {
						t.Fatalf("JSON shape = %T, want object", value)
					}
				}
			}
		})
	}
}

func TestRYOSSHOutputShapes(t *testing.T) {
	cliEnvironment(t)
	tests := []struct {
		name  string
		argv  []string
		code  int
		shape string
	}{
		{name: "list", argv: []string{"ryossh", "list"}, shape: "array"},
		{name: "pingall", argv: []string{"ryossh", "pingall"}, shape: "array"},
		{name: "console list", argv: []string{"ryossh", "console", "list"}, shape: "array"},
		{name: "console caps", argv: []string{"ryossh", "console", "caps"}, shape: "object"},
		{name: "unknown", argv: []string{"ryossh", "wat"}, code: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, gui, stdout, stderr := runCLI(t, test.argv...)
			if code != test.code || gui {
				t.Fatalf("Main = (%d, %v), want (%d, false); stderr=%q", code, gui, test.code, stderr)
			}
			if test.shape == "" {
				return
			}
			var value any
			if err := json.Unmarshal([]byte(stdout), &value); err != nil {
				t.Fatalf("invalid JSON %q: %v", stdout, err)
			}
			if test.shape == "array" {
				if _, ok := value.([]any); !ok {
					t.Fatalf("JSON shape = %T, want array", value)
				}
			} else if _, ok := value.(map[string]any); !ok {
				t.Fatalf("JSON shape = %T, want object", value)
			}
		})
	}
}

func TestRYOSSHProbeFailurePrintsJSONAndExitsOne(t *testing.T) {
	_, bin := cliEnvironment(t)
	ssh := "#!/bin/sh\nprintf 'ssh: connect to host offline port 22: Connection refused\\n' >&2\nexit 255\n"
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(ssh), 0o700); err != nil {
		t.Fatal(err)
	}
	code, gui, stdout, stderr := runCLI(t, "ryossh", "probe", "offline")
	if code != 1 || gui || stderr != "" {
		t.Fatalf("probe = code %d gui %v stderr %q, want code 1 and JSON only", code, gui, stderr)
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(stdout), &probe); err != nil {
		t.Fatalf("invalid probe JSON %q: %v", stdout, err)
	}
	if ok, exists := probe["ok"]; !exists || ok != false {
		t.Fatalf("probe JSON = %#v, want ok false", probe)
	}
}

func TestRYOSSHAskpass(t *testing.T) {
	cliEnvironment(t)
	t.Setenv("RYOPORT_ALIAS", "ally")
	t.Setenv("SSH_ASKPASS_REQUIRE", "force")
	code, gui, stdout, stderr := runCLI(t, "ryossh", "Password:")
	if code != 0 || gui || stdout != "s3cr3t\n" || stderr != "" {
		t.Fatalf("askpass = code %d gui %v stdout %q stderr %q", code, gui, stdout, stderr)
	}
}

func TestMonitorAndFetchDispatch(t *testing.T) {
	root, _ := cliEnvironment(t)
	tests := []struct {
		name       string
		argv       []string
		code       int
		stdoutPart string
		stderrPart string
	}{
		{name: "stopped stats", argv: []string{"ryovm-mon", "stats", "/missing/monitor", "-", "-"}, stdoutPart: `{"running":false}`},
		{name: "monitor usage", argv: []string{"ryovm-mon"}, code: 1, stderrPart: "usage: ryovm-mon"},
		{name: "fetch usage", argv: []string{"ryovm-fetch"}, code: 1, stdoutPart: `"message":"usage: ryovm-fetch \u003curl\u003e \u003cdest\u003e"`},
		{name: "fetch error shape", argv: []string{"ryovm-fetch", "://bad", filepath.Join(root, "download")}, code: 1, stdoutPart: `{"event":"error"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, gui, stdout, stderr := runCLI(t, test.argv...)
			if code != test.code || gui {
				t.Fatalf("Main = (%d, %v), want (%d, false)", code, gui, test.code)
			}
			if test.stdoutPart != "" && !strings.Contains(stdout, test.stdoutPart) {
				t.Fatalf("stdout %q does not contain %q", stdout, test.stdoutPart)
			}
			if test.stderrPart != "" && !strings.Contains(stderr, test.stderrPart) {
				t.Fatalf("stderr %q does not contain %q", stderr, test.stderrPart)
			}
		})
	}
}
