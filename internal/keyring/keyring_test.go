package keyring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretToolArgumentsAndStdin(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	script := filepath.Join(dir, "secret-tool")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$KEYRING_LOG\"\ncase \"$1\" in\nstore) while IFS= read -r line || [ -n \"$line\" ]; do printf '%s\\n' \"$line\"; done >> \"$KEYRING_LOG\" ;;\nlookup) printf 'hunter2\\n' ;;\nesac\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("KEYRING_LOG", log)
	if !Available() {
		t.Fatal("fake secret-tool not found")
	}
	attrs := map[string]string{"ryoport-alias": "ally", "scope": "ssh"}
	if err := Store("ryoport: ally", attrs, "secret-value"); err != nil {
		t.Fatal(err)
	}
	secret, err := Lookup(attrs)
	if err != nil || secret != "hunter2" {
		t.Fatalf("Lookup = %q, %v", secret, err)
	}
	if err := Clear(attrs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "store --label ryoport: ally ryoport-alias ally scope ssh\nsecret-value") {
		t.Fatalf("store invocation or stdin missing:\n%s", text)
	}
	if !strings.Contains(text, "lookup ryoport-alias ally scope ssh") || !strings.Contains(text, "clear ryoport-alias ally scope ssh") {
		t.Fatalf("lookup/clear invocation missing:\n%s", text)
	}
}
