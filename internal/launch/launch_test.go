package launch

import (
	"strings"
	"testing"
)

func TestTerminalArgvClassForm(t *testing.T) {
	const class = "dev.ryoku.ryoport_ssh"
	if !strings.Contains(class, ".") || strings.Contains(class, "-") {
		t.Fatalf("%q is not a GTK application id", class)
	}
	for _, terminal := range []string{"kitty", "ghostty", "foot", "alacritty"} {
		argv := terminalArgv(terminal, true, []string{"ssh", "myhost"}, class, "ssh: myhost")
		var hasID, hasBare bool
		for _, arg := range argv {
			if strings.HasSuffix(arg, "="+class) {
				hasID = true
			}
			if arg == "--class" || arg == "--app-id" {
				hasBare = true
			}
		}
		if !hasID {
			t.Errorf("%s argv = %v, want class %q passed with =", terminal, argv, class)
		}
		if hasBare {
			t.Errorf("%s argv = %v, must not pass a valueless class flag", terminal, argv)
		}
	}
	for _, arg := range terminalArgv("wezterm", true, []string{"ssh", "myhost"}, class, "ssh: myhost") {
		if strings.HasPrefix(arg, "--class") {
			t.Fatalf("wezterm argv carries a class flag: %v", arg)
		}
	}
}
