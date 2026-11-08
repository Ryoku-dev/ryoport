package settings

import (
	"strings"
	"testing"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoport/internal/update"
)

func TestUpdateStateWords(t *testing.T) {
	tests := []struct {
		name       string
		info       update.Info
		wantTitle  string
		wantDetail string
	}{
		{
			name:       "installable release",
			info:       update.Info{Current: "1.0.0", Latest: "1.1.0", Available: true, CanInstall: true},
			wantTitle:  "Ryoport 1.1.0 is available",
			wantDetail: "The update can be installed here.",
		},
		{
			name:       "distribution install preserves reason",
			info:       update.Info{Current: "1.0.0", Latest: "1.1.0", Available: true, Install: update.InstallDistro, Reason: "Run sudo pacman -Syu."},
			wantTitle:  "Ryoport 1.1.0 is available",
			wantDetail: "Run sudo pacman -Syu.",
		},
		{
			name:       "source install preserves reason",
			info:       update.Info{Current: "1.0.0", Latest: "1.1.0", Available: true, Install: update.InstallSource, Reason: "Rebuild from the checkout that installed this binary."},
			wantTitle:  "Ryoport 1.1.0 is available",
			wantDetail: "Rebuild from the checkout that installed this binary.",
		},
		{
			name:       "restart pending",
			info:       update.Info{Current: "1.0.0", Latest: "1.1.0", RestartPending: true},
			wantTitle:  "Restart to finish",
			wantDetail: "Ryoport 1.1.0 is installed",
		},
		{
			name:       "up to date",
			info:       update.Info{Current: "1.0.0", Latest: "1.0.0"},
			wantTitle:  "Ryoport is up to date",
			wantDetail: "Ryoport 1.0.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := updateStateWords(test.info)
			if got.Title != test.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, test.wantTitle)
			}
			if !strings.Contains(got.Detail, test.wantDetail) {
				t.Errorf("Detail = %q, want it to contain %q", got.Detail, test.wantDetail)
			}
		})
	}
}

func TestWithTool(t *testing.T) {
	tools := []string{"git", "python", "git"}
	withoutGit := withTool(tools, "git", false)
	if strings.Join(withoutGit, ",") != "python" {
		t.Fatalf("remove git = %v", withoutGit)
	}
	withGo := withTool(withoutGit, "go", true)
	if strings.Join(withGo, ",") != "python,go" {
		t.Fatalf("add go = %v", withGo)
	}
}

func TestVisibleSection(t *testing.T) {
	heights := []int{150, 900, 300, 300}
	tests := []struct {
		name string
		pos  layout.Position
		want int
	}{
		{"a short first section is current at the very top", layout.Position{BeforeEnd: true}, 0},
		{"next takes over once the first leaves the top third", layout.Position{Offset: 100, BeforeEnd: true}, 1},
		{"middle section holds the top third", layout.Position{First: 1, Offset: 200, BeforeEnd: true}, 1},
		{"next takes over from the middle", layout.Position{First: 1, Offset: 750, BeforeEnd: true}, 2},
		{"scrolled to the very end", layout.Position{First: 1, Offset: 700}, 3},
		{"everything fits without scrolling", layout.Position{}, 0},
	}
	for _, test := range tests {
		if got := visibleSection(test.pos, heights, 600); got != test.want {
			t.Errorf("%s: section %d, want %d", test.name, got, test.want)
		}
	}
}
