package version

import (
	"os"
	"strings"
	"testing"
)

func rootChangelog(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestVersionMatchesTopEntry(t *testing.T) {
	entries := Entries()
	if len(entries) == 0 {
		t.Fatal("changelog has no entries")
	}
	if entries[0].Version != Version {
		t.Fatalf("Version is %q, top changelog entry is %q", Version, entries[0].Version)
	}
	for _, entry := range entries {
		if len(entry.Changes) == 0 {
			t.Errorf("v%s has no changes", entry.Version)
		}
	}
}

func TestLatest(t *testing.T) {
	latest := Latest()
	if latest.Version != Version {
		t.Fatalf("Latest().Version = %q, want %q", latest.Version, Version)
	}
	if len(latest.Changes) == 0 {
		t.Fatal("Latest() has no changes")
	}
}

func TestNotesAreExactlyTopSection(t *testing.T) {
	notes := Notes()
	if notes == "" {
		t.Fatal("Notes() is empty")
	}
	var headers []string
	for line := range strings.Lines(notes) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			headers = append(headers, line)
		}
	}
	if len(headers) != 1 || headers[0] != "## v"+Version {
		t.Fatalf("Notes() headers = %q, want only %q", headers, "## v"+Version)
	}

	root := string(rootChangelog(t))
	start := strings.Index(root, "## v"+Version+"\n")
	if start < 0 {
		t.Fatal("top section header not found")
	}
	end := len(root)
	position := start + len("## v"+Version+"\n")
	for line := range strings.Lines(root[position:]) {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			end = position
			break
		}
		position += len(line)
	}
	if notes != root[start:end] {
		t.Fatalf("Notes() = %q, want %q", notes, root[start:end])
	}
}

func TestEntriesAcceptHeadingSuffix(t *testing.T) {
	original := changelog
	changelog = "# Changelog\n\n## v1.2.3 first release\n\n- One change.\n"
	t.Cleanup(func() { changelog = original })

	entries := Entries()
	if len(entries) != 1 || entries[0].Version != "1.2.3" {
		t.Fatalf("Entries() = %#v", entries)
	}
}
