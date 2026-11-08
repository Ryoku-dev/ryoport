package settings

import (
	"strings"
	"testing"

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
