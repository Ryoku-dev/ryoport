// Package build creates and imports quickemu machines.
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

// Phase identifies a create or instant-build stream event.
type Phase string

const (
	PhaseResolve   Phase = "resolve"
	PhaseDownload  Phase = "download"
	PhaseConfig    Phase = "config"
	PhaseDone      Phase = "done"
	PhaseCancelled Phase = "cancelled"
	PhaseError     Phase = "error"
	PhaseLog       Phase = "log"
)

// Event is the legacy build stream plus in-process download progress.
type Event struct {
	Phase    Phase           `json:"phase"`
	Name     string          `json:"name,omitempty"`
	Message  string          `json:"message,omitempty"`
	Line     string          `json:"line,omitempty"`
	Progress *fetch.Progress `json:"-"`
}

// Builder owns machine creation dependencies.
type Builder struct {
	VM   *vm.Manager
	HTTP *http.Client
}

func New(m *vm.Manager) *Builder {
	if m == nil {
		m = vm.New()
	}
	return &Builder{VM: m, HTTP: http.DefaultClient}
}

func (b *Builder) client() *http.Client {
	if b.HTTP != nil {
		return b.HTTP
	}
	return http.DefaultClient
}

func emitEvent(emit func(Event), event Event) {
	if emit != nil {
		emit(event)
	}
}

func failEvent(emit func(Event), err error) error {
	if errors.Is(err, context.Canceled) {
		emitEvent(emit, Event{Phase: PhaseCancelled})
		return err
	}
	emitEvent(emit, Event{Phase: PhaseError, Message: err.Error()})
	return err
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := xdg.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// RawCatalog returns quickget's JSON catalogue, refreshing its atomic cache when requested.
func (b *Builder) RawCatalog(ctx context.Context, refresh bool) ([]byte, error) {
	cache := xdg.CatalogCache()
	if !refresh {
		if data, err := readRegular(cache); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	cmd := exec.CommandContext(ctx, "quickget", "--list-json")
	data, err := cmd.Output()
	if err == nil && len(data) > 0 && json.Valid(data) {
		if writeErr := atomicWrite(cache, data, 0o644); writeErr != nil {
			return nil, writeErr
		}
		return data, nil
	}
	if cached, readErr := readRegular(cache); readErr == nil && len(cached) > 0 {
		return cached, nil
	}
	if err == nil {
		err = errors.New("quickget returned an invalid OS catalogue")
	}
	return nil, fmt.Errorf("could not fetch the OS catalogue: %w", err)
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

// OS is one grouped catalogue operating system.
type OS struct {
	OS       string              `json:"os"`
	Name     string              `json:"name"`
	PNG      string              `json:"png"`
	SVG      string              `json:"svg"`
	Releases []string            `json:"releases"`
	Editions map[string][]string `json:"editions"`
}

type catalogRow struct {
	OS      string `json:"OS"`
	Name    string `json:"Display Name"`
	PNG     string `json:"PNG"`
	SVG     string `json:"SVG"`
	Release string `json:"Release"`
	Option  string `json:"Option"`
}

// Catalog groups quickget rows in the shape consumed by the old QML catalogue.
func (b *Builder) Catalog(ctx context.Context, refresh bool) ([]OS, error) {
	data, err := b.RawCatalog(ctx, refresh)
	if err != nil {
		return nil, err
	}
	var rows []catalogRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("quickget catalogue: %w", err)
	}
	bySlug := make(map[string]int)
	systems := make([]OS, 0)
	for _, row := range rows {
		if row.OS == "" || row.Release == "" {
			continue
		}
		index, exists := bySlug[row.OS]
		if !exists {
			name := row.Name
			if name == "" {
				name = row.OS
			}
			index = len(systems)
			bySlug[row.OS] = index
			systems = append(systems, OS{OS: row.OS, Name: name, PNG: row.PNG, SVG: row.SVG, Editions: make(map[string][]string)})
		}
		o := &systems[index]
		if _, exists := o.Editions[row.Release]; !exists {
			o.Releases = append(o.Releases, row.Release)
			o.Editions[row.Release] = []string{}
		}
		if row.Option != "" && !contains(o.Editions[row.Release], row.Option) {
			o.Editions[row.Release] = append(o.Editions[row.Release], row.Option)
		}
	}
	sort.SliceStable(systems, func(i, j int) bool {
		return strings.ToLower(systems[i].Name) < strings.ToLower(systems[j].Name)
	})
	return systems, nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

var developmentRelease = regexp.MustCompile(`(?i)(^|[-_])(daily|weekly|nightly|unstable|testing|preview|beta|canary|sid)([-_]|$)|^rc`)

func DefaultRelease(o OS) string {
	for i := len(o.Releases) - 1; i >= 0; i-- {
		if !developmentRelease.MatchString(o.Releases[i]) {
			return o.Releases[i]
		}
	}
	if len(o.Releases) > 0 {
		return o.Releases[len(o.Releases)-1]
	}
	return ""
}

func DefaultEdition(o OS, release string) string {
	editions := o.Editions[release]
	for _, preferred := range []string{"standard", "default", "gnome", "plasma", "kde"} {
		if contains(editions, preferred) {
			return preferred
		}
	}
	if len(editions) > 0 {
		return editions[0]
	}
	return ""
}
