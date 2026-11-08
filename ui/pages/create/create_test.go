package create

import (
	"reflect"
	"testing"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/store"
)

func TestFilterCatalogByNameAndSlug(t *testing.T) {
	systems := []build.OS{
		{OS: "ubuntu", Name: "Ubuntu"},
		{OS: "opensuse", Name: "openSUSE Tumbleweed"},
		{OS: "archlinux", Name: "Arch Linux"},
	}
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "empty keeps all", query: "  ", want: []string{"ubuntu", "opensuse", "archlinux"}},
		{name: "display name", query: "TUMBLE", want: []string{"opensuse"}},
		{name: "slug", query: "archlinux", want: []string{"archlinux"}},
		{name: "partial slug", query: "suse", want: []string{"opensuse"}},
		{name: "no match", query: "windows", want: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSystems := filterCatalog(systems, test.query)
			got := make([]string, len(gotSystems))
			for i, system := range gotSystems {
				got[i] = system.OS
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("filterCatalog(%q) = %v, want %v", test.query, got, test.want)
			}
		})
	}
}

func TestFilterCatalogRanksUsefulMatchesFirst(t *testing.T) {
	systems := []build.OS{
		{OS: "my-debian", Name: "Custom appliance"},
		{OS: "debian-testing", Name: "Debian testing"},
		{OS: "bookworm", Name: "GNU Debian Bookworm"},
	}
	gotSystems := filterCatalog(systems, "debian")
	got := make([]string, len(gotSystems))
	for i, system := range gotSystems {
		got[i] = system.OS
	}
	want := []string{"debian-testing", "bookworm", "my-debian"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterCatalog() = %v, want %v", got, want)
	}
}

func TestAvailableItemKindsForHost(t *testing.T) {
	tests := []struct {
		name         string
		host         store.Host
		hasTemplates bool
		want         []itemKind
	}{
		{
			name: "base host",
			want: []itemKind{itemCatalog, itemInstant, itemImport, itemServer, itemConsole},
		},
		{
			name:         "templates available",
			hasTemplates: true,
			want:         []itemKind{itemCatalog, itemInstant, itemImport, itemSpawn, itemServer, itemConsole},
		},
		{
			name: "passthrough available",
			host: store.Host{Passthrough: true},
			want: []itemKind{itemCatalog, itemInstant, itemImport, itemPassthrough, itemServer, itemConsole},
		},
		{
			name:         "full host",
			host:         store.Host{Passthrough: true},
			hasTemplates: true,
			want:         []itemKind{itemCatalog, itemInstant, itemImport, itemSpawn, itemPassthrough, itemServer, itemConsole},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := availableItemKinds(test.host, test.hasTemplates); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("availableItemKinds() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNameErrorUsesVMNameRules(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{name: "ubuntu-24.04", ok: true},
		{name: "dev_box", ok: true},
		{name: "", ok: false},
		{name: ".", ok: false},
		{name: "..", ok: false},
		{name: "has a space", ok: false},
		{name: "path/name", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nameError(test.name) == ""; got != test.ok {
				t.Fatalf("nameError(%q) valid = %v, want %v", test.name, got, test.ok)
			}
		})
	}
}

func TestISOErrorRequiresISOExtension(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
	}{
		{path: "/images/fedora.iso", ok: true},
		{path: "/images/WINDOWS.ISO", ok: true},
		{path: "  relative.iso  ", ok: true},
		{path: "", ok: false},
		{path: "/images/disk.img", ok: false},
		{path: "/images/iso", ok: false},
		{path: "/images/system.iso.part", ok: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := isoError(test.path) == ""; got != test.ok {
				t.Fatalf("isoError(%q) valid = %v, want %v", test.path, got, test.ok)
			}
		})
	}
}
