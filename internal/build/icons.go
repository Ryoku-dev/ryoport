package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const iconTTL = 14 * 24 * time.Hour

var iconSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

var simpleIconAliases = map[string]string{
	"alma": "almalinux", "alpine": "alpinelinux", "kali": "kalilinux",
	"macos": "apple", "void": "voidlinux", "garuda": "garudalinux",
	"ubuntu-server": "ubuntu", "centos-stream": "centos",
	"parrotsec": "parrotsecurity", "proxmox-ve": "proxmox",
	"lmde": "linuxmint", "windows": "", "windows-server": "",
}

var distroSlugs = [...]struct {
	prefix string
	slug   string
}{
	{"arch linux", "archlinux"},
	{"ubuntu", "ubuntu"},
	{"debian", "debian"},
	{"fedora", "fedora"},
	{"linux mint", "linuxmint"},
	{"alpine", "alpine"},
	{"centos", "centos-stream"},
	{"rocky", "rockylinux"},
	{"almalinux", "alma"},
	{"opensuse", "opensuse"},
	{"nixos", "nixos"},
	{"void", "void"},
	{"gentoo", "gentoo"},
	{"manjaro", "manjaro"},
	{"pop!_os", "popos"},
	{"kali", "kali"},
	{"proxmox", "proxmox-ve"},
	{"endeavouros", "endeavouros"},
	{"cachyos", "cachyos"},
	{"elementary", "elementary"},
	{"zorin", "zorin"},
}

// DistroSlug maps an os-release display name to the catalogue logo slug.
func DistroSlug(distro string) string {
	name := strings.ToLower(strings.TrimSpace(distro))
	for _, candidate := range distroSlugs {
		if strings.HasPrefix(name, candidate.prefix) {
			return candidate.slug
		}
	}
	return ""
}

// Icon resolves and caches a logo for one catalogue OS.
func (b *Builder) Icon(ctx context.Context, osName string) (string, error) {
	if !iconSlug.MatchString(osName) {
		return "", fmt.Errorf("invalid OS slug %q", osName)
	}
	dir := xdg.IconCache()
	if err := xdg.EnsureDir(dir); err != nil {
		return "", err
	}
	svg := filepath.Join(dir, osName+".svg")
	png := filepath.Join(dir, osName+".png")
	miss := filepath.Join(dir, osName+".miss")
	if data, err := os.ReadFile(svg); err == nil && strings.Contains(string(data), `fill="#e6dccb" fill=`) {
		_ = os.Remove(svg)
	}
	if path := freshIcon(svg, png); path != "" {
		return path, nil
	}
	if fresh(miss) {
		return staleIcon(svg, png), nil
	}

	simple := osName
	if alias, exists := simpleIconAliases[osName]; exists {
		simple = alias
	}
	if simple != "" {
		data, err := b.getIcon(ctx, "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/"+simple+".svg")
		if err == nil && len(data) > 0 {
			data = tintSVG(data)
			if err := atomicWrite(svg, data, 0o644); err != nil {
				return "", err
			}
			_ = os.Remove(png)
			_ = os.Remove(miss)
			return svg, nil
		}
	}
	if info, err := os.Lstat(xdg.CatalogCache()); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return staleIcon(svg, png), nil
	}

	svgURL, pngURL := b.catalogIconURLs(osName)
	for _, source := range []struct {
		url, path string
	}{{svgURL, svg}, {pngURL, png}} {
		if source.url == "" {
			continue
		}
		data, err := b.getIcon(ctx, source.url)
		if err != nil || len(data) == 0 {
			continue
		}
		if err := atomicWrite(source.path, data, 0o644); err != nil {
			return "", err
		}
		if source.path == svg {
			_ = os.Remove(png)
		}
		_ = os.Remove(miss)
		return source.path, nil
	}
	if ctx.Err() != nil {
		return staleIcon(svg, png), ctx.Err()
	}
	if err := atomicWrite(miss, nil, 0o644); err != nil {
		return staleIcon(svg, png), err
	}
	return staleIcon(svg, png), nil
}

func freshIcon(paths ...string) string {
	for _, path := range paths {
		if fresh(path) {
			if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
				return path
			}
		}
	}
	return ""
}

func staleIcon(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return path
		}
	}
	return ""
}

func fresh(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && time.Since(info.ModTime()) < iconTTL
}

func tintSVG(data []byte) []byte {
	text := string(data)
	start := strings.Index(text, "<svg ")
	if start < 0 {
		return data
	}
	end := strings.Index(text[start:], ">")
	if end >= 0 && strings.Contains(text[start:start+end], " fill=") {
		return data
	}
	return []byte(text[:start] + `<svg fill="#e6dccb" ` + text[start+5:])
}

func (b *Builder) getIcon(ctx context.Context, source string) ([]byte, error) {
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, errors.New("invalid icon URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ryovm-fetch/1")
	resp, err := b.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("icon server replied %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func (b *Builder) catalogIconURLs(osName string) (string, string) {
	data, err := readRegular(xdg.CatalogCache())
	if err != nil {
		return "", ""
	}
	var rows []catalogRow
	if json.Unmarshal(data, &rows) != nil {
		return "", ""
	}
	for _, row := range rows {
		if row.OS == osName {
			return row.SVG, row.PNG
		}
	}
	return "", ""
}

// Prefetch resolves logos with at most twelve concurrent requests.
func (b *Builder) Prefetch(ctx context.Context, oses []string) map[string]string {
	result := make(map[string]string)
	seen := make(map[string]bool)
	semaphore := make(chan struct{}, 12)
	var mutex sync.Mutex
	var group sync.WaitGroup
	for _, osName := range oses {
		if seen[osName] {
			continue
		}
		seen[osName] = true
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			path, err := b.Icon(ctx, osName)
			if err == nil && path != "" {
				mutex.Lock()
				result[osName] = path
				mutex.Unlock()
			}
		}()
	}
	group.Wait()
	return result
}
