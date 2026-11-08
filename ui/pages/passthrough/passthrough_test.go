package passthrough

import (
	"path/filepath"
	"reflect"
	"testing"

	backend "github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/ui/pages/common"
)

func TestPassthroughProof(t *testing.T) {
	machine := backend.Machine{Name: "win11-gpu", VCPUs: 6, RAMMB: 16 * 1024}
	want := []common.Span{
		{Text: "A passthrough machine, opens in Looking Glass. "},
		{Text: "6", Mono: true},
		{Text: " cores, "},
		{Text: "16", Mono: true},
		{Text: " GB."},
	}
	if got := passthroughProof(machine); !reflect.DeepEqual(got, want) {
		t.Fatalf("passthroughProof() = %#v, want %#v", got, want)
	}
}

func TestValidatePassthroughDraftAcceptsBackendLimits(t *testing.T) {
	for _, options := range []backend.CreateOptions{
		{Name: "guest_1", ISO: filepath.Join(string(filepath.Separator), "images", "guest.iso"), OS: "linux", DiskGB: 16, RAMMB: 2 * 1024},
		{Name: "WIN-11", ISO: filepath.Join(string(filepath.Separator), "images", "windows.iso"), OS: "windows", DiskGB: 1024, RAMMB: 128 * 1024},
	} {
		if errs := validatePassthroughDraft(options); errs.Any() {
			t.Fatalf("valid options rejected: %+v", errs)
		}
	}
}

func TestValidatePassthroughDraftRejectsBackendLimits(t *testing.T) {
	valid := backend.CreateOptions{
		Name:   "guest_1",
		ISO:    filepath.Join(string(filepath.Separator), "images", "guest.iso"),
		OS:     "linux",
		DiskGB: 64,
		RAMMB:  16 * 1024,
	}
	tests := []struct {
		name   string
		mutate func(*backend.CreateOptions)
		check  func(passthroughDraftErrors) bool
	}{
		{name: "name required", mutate: func(o *backend.CreateOptions) { o.Name = "" }, check: func(e passthroughDraftErrors) bool { return e.Name != "" }},
		{name: "name characters", mutate: func(o *backend.CreateOptions) { o.Name = "bad name" }, check: func(e passthroughDraftErrors) bool { return e.Name != "" }},
		{name: "absolute iso", mutate: func(o *backend.CreateOptions) { o.ISO = "guest.iso" }, check: func(e passthroughDraftErrors) bool { return e.ISO != "" }},
		{name: "guest os", mutate: func(o *backend.CreateOptions) { o.OS = "bsd" }, check: func(e passthroughDraftErrors) bool { return e.OS != "" }},
		{name: "disk low", mutate: func(o *backend.CreateOptions) { o.DiskGB = 15 }, check: func(e passthroughDraftErrors) bool { return e.Disk != "" }},
		{name: "disk high", mutate: func(o *backend.CreateOptions) { o.DiskGB = 1025 }, check: func(e passthroughDraftErrors) bool { return e.Disk != "" }},
		{name: "ram low", mutate: func(o *backend.CreateOptions) { o.RAMMB = 1024 }, check: func(e passthroughDraftErrors) bool { return e.RAM != "" }},
		{name: "ram high", mutate: func(o *backend.CreateOptions) { o.RAMMB = 129 * 1024 }, check: func(e passthroughDraftErrors) bool { return e.RAM != "" }},
		{name: "ram whole gb", mutate: func(o *backend.CreateOptions) { o.RAMMB = 2500 }, check: func(e passthroughDraftErrors) bool { return e.RAM != "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := valid
			test.mutate(&options)
			errs := validatePassthroughDraft(options)
			if !test.check(errs) {
				t.Fatalf("invalid options accepted: %+v", options)
			}
		})
	}
}
