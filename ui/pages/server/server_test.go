package server

import (
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/store"
)

func TestProofSentenceOnline(t *testing.T) {
	ally := &store.Server{
		Host:  remote.Host{Alias: "ally", HostName: "192.168.110.8", User: "neur0map"},
		Reach: &remote.Reach{Alias: "ally", Up: true, SSHUp: true, RTTMs: 23},
		Probe: &remote.Probe{Alias: "ally", OK: true, UptimeS: 12 * 24 * 60 * 60, Distro: "Ubuntu 24.04"},
	}
	peers := map[string]*store.Server{
		"ally":  ally,
		"prowl": &store.Server{Host: remote.Host{Alias: "prowl", HostName: "192.168.110.8"}},
		"other": &store.Server{Host: remote.Host{Alias: "other", HostName: "10.0.0.5"}},
	}
	spans := proofSentence(ally, peers, model.StateOnline, time.Time{}, time.Time{})
	if len(spans) != 1 {
		t.Fatalf("proofSentence returned %d spans, want 1", len(spans))
	}
	want := "Online for 12 days. 23 ms from here. Ubuntu 24.04 on the same box as prowl."
	if spans[0].Text != want {
		t.Fatalf("proofSentence() = %q, want %q", spans[0].Text, want)
	}
}

func TestProofSentenceNeedsAuth(t *testing.T) {
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	server := &store.Server{
		Host:  remote.Host{Alias: "mika-server", HostName: "203.0.113.70", User: "root"},
		Reach: &remote.Reach{Alias: "mika-server", Up: true, RTTMs: 68},
		Probe: &remote.Probe{Alias: "mika-server", NeedAuth: true},
	}
	spans := proofSentence(server, nil, model.StateNeedsAuth, now.Add(-2*time.Minute), now)
	want := "Reachable, 68 ms from here. Sign-in as root failed 2 min ago."
	if len(spans) != 1 || spans[0].Text != want {
		t.Fatalf("proofSentence() = %#v, want %q", spans, want)
	}
}

func TestProofSentenceFailedUnits(t *testing.T) {
	for _, test := range []struct {
		count int
		want  string
	}{
		{count: 1, want: "Online. 1 failed unit."},
		{count: 3, want: "Online. 3 failed units."},
	} {
		server := &store.Server{
			Host:  remote.Host{Alias: "ally", HostName: "192.168.110.8"},
			Reach: &remote.Reach{Alias: "ally", Up: true, SSHUp: true, RTTMs: -1},
			Probe: &remote.Probe{Alias: "ally", OK: true, FailedUnits: test.count},
		}
		spans := proofSentence(server, nil, model.StateDegraded, time.Time{}, time.Time{})
		if len(spans) != 1 || spans[0].Text != test.want {
			t.Errorf("proofSentence() with %d failed units = %#v, want %q", test.count, spans, test.want)
		}
	}
}

func TestValidateDraft(t *testing.T) {
	tests := []struct {
		name  string
		input validationInput
		want  validationErrors
	}{
		{
			name:  "valid",
			input: validationInput{Alias: "build_01", Host: "build.example.com", Port: "2222", Tunnel: "L:8080:127.0.0.1:80"},
		},
		{
			name:  "missing required fields",
			input: validationInput{},
			want: validationErrors{
				Alias: "Enter an alias.",
				Host:  "Enter a host name or address.",
				Port:  "Port must be from 1 to 65535.",
			},
		},
		{
			name:  "bad alias",
			input: validationInput{Alias: "-bad alias", Host: "host", Port: "22"},
			want:  validationErrors{Alias: "Use letters, numbers, dots, underscores, or hyphens."},
		},
		{
			name:  "bad host",
			input: validationInput{Alias: "host", Host: "-bad host", Port: "22"},
			want:  validationErrors{Host: "Host names cannot contain spaces or start with a hyphen."},
		},
		{
			name:  "port above range",
			input: validationInput{Alias: "host", Host: "host", Port: "65536"},
			want:  validationErrors{Port: "Port must be from 1 to 65535."},
		},
		{
			name:  "bad tunnel",
			input: validationInput{Alias: "host", Host: "host", Port: "22", Tunnel: "L:8080"},
			want:  validationErrors{Tunnel: "Use L:port:host:port, R:port:host:port, or D:port."},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := validateDraft(test.input)
			if got != test.want {
				t.Fatalf("validateDraft(%#v) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}

func TestValidateTunnelSpec(t *testing.T) {
	for _, spec := range []string{
		"L:8080:127.0.0.1:80",
		"R:127.0.0.1:9000:localhost:9000",
		"D:1080",
	} {
		if errText := validateTunnelSpec(spec); errText != "" {
			t.Errorf("validateTunnelSpec(%q) = %q", spec, errText)
		}
	}
	for _, spec := range []string{"", "8080:host:80", "D:70000", "L:80:bad host:80"} {
		if errText := validateTunnelSpec(spec); errText == "" {
			t.Errorf("validateTunnelSpec(%q) accepted an invalid spec", spec)
		}
	}
}
