// Package version exposes Ryoport's release version and changelog.
package version

import (
	_ "embed"
	"regexp"
	"strings"
)

const Version = "1.0.0"

//go:embed CHANGELOG.md
var changelog string

var headingPattern = regexp.MustCompile(`^## v([0-9]+\.[0-9]+\.[0-9]+)(?:\s+.*)?$`)

type Entry struct {
	Version string
	Changes []string
}

func Entries() []Entry {
	var entries []Entry
	var current *Entry
	for line := range strings.Lines(changelog) {
		line = strings.TrimSpace(line)
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			entries = append(entries, Entry{Version: match[1]})
			current = &entries[len(entries)-1]
			continue
		}
		if current != nil && strings.HasPrefix(line, "- ") {
			current.Changes = append(current.Changes, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		}
	}
	return entries
}

func Latest() Entry {
	entries := Entries()
	if len(entries) == 0 {
		return Entry{}
	}
	return entries[0]
}

func Notes() string {
	start := -1
	end := len(changelog)
	position := 0
	for line := range strings.Lines(changelog) {
		trimmed := strings.TrimSpace(line)
		if headingPattern.MatchString(trimmed) {
			if start >= 0 {
				end = position
				break
			}
			start = position
		}
		position += len(line)
	}
	if start < 0 {
		return ""
	}
	return changelog[start:end]
}
