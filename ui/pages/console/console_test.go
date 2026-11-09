package console

import (
	"reflect"
	"testing"

	backend "github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

func TestConsoleProof(t *testing.T) {
	tests := []struct {
		name    string
		profile backend.Profile
		reach   *backend.Reach
		want    []common.Span
	}{
		{
			name:    "serial",
			profile: backend.Profile{Kind: "serial", Device: "/dev/ttyUSB0", Baud: 115200},
			want: []common.Span{
				{Text: "Serial on "},
				{Text: "/dev/ttyUSB0", Mono: true},
				{Text: " at "},
				{Text: "115200", Mono: true},
			},
		},
		{
			name:    "serial not plugged in",
			profile: backend.Profile{Kind: "serial", Device: "/dev/ttyUSB0", Baud: 115200},
			reach:   &backend.Reach{Error: "device not present: /dev/ttyUSB0"},
			want: []common.Span{
				{Text: "Serial on "},
				{Text: "/dev/ttyUSB0", Mono: true},
				{Text: " at "},
				{Text: "115200", Mono: true},
				{Text: ". Not plugged in."},
			},
		},
		{
			name:    "reachable rdp",
			profile: backend.Profile{Kind: "rdp", Address: "10.0.0.5"},
			reach:   &backend.Reach{Up: true, RTTMs: 4},
			want: []common.Span{
				{Text: "RDP to "},
				{Text: "10.0.0.5", Mono: true},
				{Text: ", "},
				{Text: "4 ms", Mono: true},
			},
		},
		{
			name:    "unreachable vnc",
			profile: backend.Profile{Kind: "vnc", Address: "desk.home"},
			reach:   &backend.Reach{Up: false},
			want: []common.Span{
				{Text: "VNC to "},
				{Text: "desk.home", Mono: true},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := consoleProof(test.profile, test.reach); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("consoleProof() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestConsoleConnectBlockerExplainsMissingSerialDevice(t *testing.T) {
	profile := backend.Profile{Kind: "serial", Device: "/dev/ttyUSB0", Baud: 115200}
	reach := &backend.Reach{Error: "device not present: /dev/ttyUSB0"}
	if got := consoleConnectBlocker(backend.Caps{Serial: true}, profile, reach); got != "Not plugged in." {
		t.Fatalf("consoleConnectBlocker() = %q, want %q", got, "Not plugged in.")
	}
}

func TestParseConsoleDraftAcceptsKindFields(t *testing.T) {
	tests := []struct {
		name string
		kind string
		args []string
		port int
		baud int
	}{
		{name: "serial", kind: "serial", args: []string{"/dev/ttyUSB0", "115200", "", ""}, baud: 115200},
		{name: "telnet", kind: "telnet", args: []string{"", "", "router.home", "23"}, port: 23},
		{name: "rdp", kind: "rdp", args: []string{"", "", "10.0.0.5", "3389"}, port: 3389},
		{name: "vnc", kind: "vnc", args: []string{"", "", "desk_home", "5900"}, port: 5900},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile, errs := parseConsoleDraft("id", test.kind, "Console", test.args[0], test.args[1], test.args[2], test.args[3], "alice", "HQ", true)
			if errs.Any() {
				t.Fatalf("valid draft rejected: %+v", errs)
			}
			if profile.Port != test.port || profile.Baud != test.baud {
				t.Fatalf("parsed profile = %+v", profile)
			}
		})
	}
}

func TestParseConsoleDraftRejectsInvalidConnectionFields(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		dev   string
		baud  string
		host  string
		port  string
		check func(consoleDraftErrors) bool
	}{
		{name: "kind", kind: "ssh", check: func(e consoleDraftErrors) bool { return e.Kind != "" }},
		{name: "device path", kind: "serial", dev: "ttyUSB0", baud: "115200", check: func(e consoleDraftErrors) bool { return e.Device != "" }},
		{name: "device spaces", kind: "serial", dev: "/dev/tty USB0", baud: "115200", check: func(e consoleDraftErrors) bool { return e.Device != "" }},
		{name: "baud low", kind: "serial", dev: "/dev/ttyUSB0", baud: "29", check: func(e consoleDraftErrors) bool { return e.Baud != "" }},
		{name: "baud high", kind: "serial", dev: "/dev/ttyUSB0", baud: "4000001", check: func(e consoleDraftErrors) bool { return e.Baud != "" }},
		{name: "address", kind: "rdp", host: "host:3389", port: "3389", check: func(e consoleDraftErrors) bool { return e.Address != "" }},
		{name: "port low", kind: "telnet", host: "router", port: "0", check: func(e consoleDraftErrors) bool { return e.Port != "" }},
		{name: "port high", kind: "vnc", host: "desk", port: "65536", check: func(e consoleDraftErrors) bool { return e.Port != "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := parseConsoleDraft("", test.kind, "Console", test.dev, test.baud, test.host, test.port, "", "", false)
			if !test.check(errs) {
				t.Fatalf("invalid draft accepted: %+v", errs)
			}
		})
	}
}

func TestConsoleDraftFieldErrorWaitsForEditedBlur(t *testing.T) {
	var state consoleFieldValidation
	state.reset("")
	_, errs := parseConsoleDraft("", "serial", "", "/dev/ttyUSB0", "115200", "", "", "", "", false)
	if got := state.visible(errs.Name); got != "" {
		t.Fatalf("fresh field error = %q, want none", got)
	}

	state.observe("", true)
	state.observe("bad\nname", true)
	_, errs = parseConsoleDraft("", "serial", "bad\nname", "/dev/ttyUSB0", "115200", "", "", "", "", false)
	if got := state.visible(errs.Name); got != "" {
		t.Fatalf("focused edited field error = %q, want none", got)
	}
	state.observe("bad\nname", false)
	if got := state.visible(errs.Name); got == "" {
		t.Fatal("edited field did not show its error after losing focus")
	}
}
