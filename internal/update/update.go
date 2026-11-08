// Package update checks and installs signed Ryoport release artifacts.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/version"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const (
	defaultAPI  = "https://api.github.com/repos/ryoku-dev/ryoport/releases/latest"
	repository  = "https://github.com/ryoku-dev/ryoport"
	maxMetadata = 1024 * 1024
	maxPackage  = 256 * 1024 * 1024
	maxChecksum = 4 * 1024
)

var releaseTag = regexp.MustCompile(`^v1\.(0|[1-9][0-9]*)\.[0-9]$`)

type Install int

const (
	InstallPacman Install = iota
	InstallTarball
	InstallDistro
	InstallSource
)

type Info struct {
	Current        string  `json:"current"`
	Latest         string  `json:"latest,omitempty"`
	Available      bool    `json:"available"`
	URL            string  `json:"url"`
	Notes          string  `json:"notes,omitempty"`
	Install        Install `json:"install"`
	CanInstall     bool    `json:"canInstall"`
	Reason         string  `json:"reason,omitempty"`
	RestartPending bool    `json:"restartPending"`
}

type Client struct {
	HTTP *http.Client
	API  string

	runner commandRunner
	detect func() (Install, string)
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Body       string  `json:"body"`
	HTMLURL    string  `json:"html_url"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

type installedMarker struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

type commandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(context.Context, string, ...string) error
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(name), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", filepath.Base(name), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Detect() (Install, string) {
	exe, err := os.Executable()
	if err != nil {
		return InstallSource, "unknown executable"
	}
	exe = strings.TrimSuffix(exe, " (deleted)")
	runner := execRunner{}
	if info, err := os.Stat("/usr/bin/pacman"); err == nil && info.Mode().IsRegular() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, queryErr := runner.Output(ctx, "/usr/bin/pacman", "-Qoq", exe)
		cancel()
		if queryErr == nil && strings.TrimSpace(string(out)) == "ryoport" {
			return InstallPacman, "pacman"
		}
	}
	local := filepath.Join(xdg.Home(), ".local")
	if pathWithin(exe, filepath.Join(local, "bin")) {
		if marker, err := readMarker(); err == nil && marker.Version != "" && len(marker.Files) > 0 {
			return InstallTarball, markerPath()
		}
	}
	if strings.HasPrefix(filepath.Clean(exe), "/nix/store/") {
		return InstallDistro, "nix"
	}
	if rpm, err := exec.LookPath("rpm"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, queryErr := runner.Output(ctx, rpm, "-qf", exe)
		cancel()
		if queryErr == nil {
			return InstallDistro, "dnf"
		}
	}
	if xbps, err := exec.LookPath("xbps-query"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, queryErr := runner.Output(ctx, xbps, "-o", exe)
		cancel()
		if queryErr == nil {
			return InstallDistro, "xbps"
		}
	}
	return InstallSource, exe
}

func (client *Client) Check(ctx context.Context) (Info, error) {
	release, err := client.fetchRelease(ctx)
	if err != nil {
		return Info{}, err
	}
	return client.releaseInfo(ctx, release), nil
}

func (client *Client) Apply(ctx context.Context, requested Info, progress func(done, total int64)) error {
	release, err := client.fetchRelease(ctx)
	if err != nil {
		return err
	}
	fresh := client.releaseInfo(ctx, release)
	if requested.Latest == "" || requested.Latest != fresh.Latest {
		return errors.New("the latest release changed; check for updates again")
	}
	if !fresh.Available && !fresh.RestartPending {
		return errors.New("this release is already running, or a newer release is installed")
	}
	if fresh.RestartPending {
		return nil
	}
	if !fresh.CanInstall {
		if fresh.Reason != "" {
			return errors.New(fresh.Reason)
		}
		return errors.New("self update is not available for this installation")
	}
	packageName, checksumName := assetNames(fresh.Install, fresh.Latest)
	packageAsset, okPackage := findAsset(release.Assets, packageName)
	checksumAsset, okChecksum := findAsset(release.Assets, checksumName)
	if !okPackage || !okChecksum {
		return errors.New("the release assets changed; check for updates again")
	}
	if err := xdg.EnsureDir(xdg.UpdateDir()); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(xdg.UpdateDir(), "ryoport-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o700); err != nil {
		return err
	}
	checksumBytes, err := client.downloadBytes(ctx, checksumAsset.URL, maxChecksum)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}
	expected, err := parseChecksum(checksumBytes, packageName)
	if err != nil {
		return err
	}
	packagePath := filepath.Join(staging, packageName)
	if err := client.downloadFile(ctx, packageAsset.URL, packagePath, expected, progress); err != nil {
		return err
	}
	switch fresh.Install {
	case InstallPacman:
		return client.applyPacman(ctx, packagePath, fresh.Latest)
	case InstallTarball:
		return client.applyTarball(ctx, packagePath, fresh.Latest, staging)
	default:
		return errors.New(fresh.Reason)
	}
}

func Relaunch() error {
	binary, err := exec.LookPath("ryoport")
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "--wait-pid", strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (client *Client) releaseInfo(ctx context.Context, release release) Info {
	install, manager := client.detectInstall()
	info := Info{
		Current: version.Version,
		URL:     repository + "/releases",
		Install: install,
	}
	if release.Draft || release.Prerelease || !releaseTag.MatchString(release.Tag) {
		return info
	}
	info.Latest = strings.TrimPrefix(release.Tag, "v")
	info.Available = compareVersions(info.Latest, info.Current) > 0
	info.Notes = release.Body
	if release.HTMLURL != "" {
		info.URL = release.HTMLURL
	} else {
		info.URL = repository + "/releases/tag/" + release.Tag
	}
	packageName, checksumName := assetNames(install, info.Latest)
	packageAsset, packageOK := findAsset(release.Assets, packageName)
	checksumAsset, checksumOK := findAsset(release.Assets, checksumName)
	assetsOK := packageOK && checksumOK && packageAsset.URL != "" && checksumAsset.URL != "" &&
		packageAsset.Size > 0 && packageAsset.Size <= maxPackage && checksumAsset.Size > 0 && checksumAsset.Size <= maxChecksum
	if !assetsOK {
		info.Reason = "This release has no complete update artifact and checksum for this installation."
	}
	switch install {
	case InstallPacman, InstallTarball:
		info.CanInstall = assetsOK
	case InstallDistro:
		info.Reason = "Updates for this install come from " + manager
	case InstallSource:
		info.Reason = "Source builds cannot update themselves"
	}
	if info.Available {
		if installed, err := client.installedVersion(ctx, install, manager); err == nil && installed == info.Latest {
			info.RestartPending = true
		}
	}
	return info
}

func (client *Client) fetchRelease(ctx context.Context) (release, error) {
	api := client.API
	if api == "" {
		api = defaultAPI
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Ryoport/"+version.Version)
	response, err := client.httpClient().Do(request)
	if err != nil {
		return release{}, fmt.Errorf("contact GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub release request returned %s", response.Status)
	}
	body, err := readBounded(response.Body, maxMetadata)
	if err != nil {
		return release{}, err
	}
	var result release
	if err := json.Unmarshal(body, &result); err != nil {
		return release{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	return result, nil
}

func (client *Client) downloadBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	response, err := client.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.ContentLength > limit {
		return nil, errors.New("download exceeds the allowed size")
	}
	return readBounded(response.Body, limit)
}

func (client *Client) downloadFile(ctx context.Context, url, path, expected string, progress func(int64, int64)) error {
	response, err := client.get(ctx, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.ContentLength > maxPackage {
		return errors.New("release package is too large")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	var done int64
	buffer := make([]byte, 64*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			done += int64(n)
			if done > maxPackage {
				return errors.New("release package is too large")
			}
			if _, err := file.Write(buffer[:n]); err != nil {
				return err
			}
			if _, err := hash.Write(buffer[:n]); err != nil {
				return err
			}
			if progress != nil {
				progress(done, response.ContentLength)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if done == 0 || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("package SHA-256 verification failed; nothing was installed")
	}
	return file.Sync()
}

func (client *Client) get(ctx context.Context, url string) (*http.Response, error) {
	if url == "" {
		return nil, errors.New("release asset has no download URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Ryoport/"+version.Version)
	response, err := client.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("download returned %s", response.Status)
	}
	return response, nil
}

func (client *Client) httpClient() *http.Client {
	if client.HTTP != nil {
		return client.HTTP
	}
	return &http.Client{Timeout: 15 * time.Minute}
}

func (client *Client) commandRunner() commandRunner {
	if client.runner != nil {
		return client.runner
	}
	return execRunner{}
}

func (client *Client) detectInstall() (Install, string) {
	if client.detect != nil {
		return client.detect()
	}
	return Detect()
}

func (client *Client) installedVersion(ctx context.Context, install Install, manager string) (string, error) {
	switch install {
	case InstallPacman:
		out, err := client.commandRunner().Output(ctx, "/usr/bin/pacman", "-Q", "ryoport")
		if err != nil {
			return "", err
		}
		fields := strings.Fields(string(out))
		if len(fields) != 2 || fields[0] != "ryoport" {
			return "", errors.New("unexpected pacman package identity")
		}
		value := strings.TrimPrefix(fields[1], "1:")
		return strings.TrimSuffix(value, "-1"), nil
	case InstallTarball:
		marker, err := readMarker()
		return marker.Version, err
	case InstallDistro:
		switch manager {
		case "dnf":
			out, err := client.commandRunner().Output(ctx, "rpm", "-q", "--qf", "%{VERSION}", "ryoport")
			return strings.TrimSpace(string(out)), err
		case "xbps":
			out, err := client.commandRunner().Output(ctx, "xbps-query", "-p", "pkgver", "ryoport")
			if err != nil {
				return "", err
			}
			value := strings.TrimSpace(string(out))
			value = strings.TrimPrefix(value, "ryoport-")
			if cut := strings.LastIndexByte(value, '_'); cut >= 0 {
				value = value[:cut]
			}
			return value, nil
		}
	}
	return "", errors.New("installed version is unavailable")
}

func assetNames(install Install, version string) (string, string) {
	var name string
	switch install {
	case InstallPacman:
		name = "ryoport-" + version + "-1-x86_64.pkg.tar.zst"
	case InstallTarball:
		name = "ryoport-" + version + "-linux-x86_64.tar.gz"
	default:
		name = "ryoport-" + version + ".tar.gz"
	}
	return name, name + ".sha256"
}

func findAsset(assets []asset, name string) (asset, bool) {
	for _, candidate := range assets {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return asset{}, false
}

func parseChecksum(data []byte, name string) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) != 2 || fields[1] != name || len(fields[0]) != sha256.Size*2 {
		return "", errors.New("invalid package checksum: expected one SHA-256 entry naming the release file")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", errors.New("invalid package checksum: expected a hexadecimal SHA-256 digest")
	}
	return strings.ToLower(fields[0]), nil
}

func compareVersions(left, right string) int {
	parse := func(value string) [3]int {
		value = strings.TrimPrefix(value, "v")
		parts := strings.Split(value, ".")
		var result [3]int
		for i := range min(3, len(parts)) {
			result[i], _ = strconv.Atoi(parts[i])
		}
		return result
	}
	a, b := parse(left), parse(right)
	for i := range 3 {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response exceeds the allowed size")
	}
	return data, nil
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func markerPath() string {
	return filepath.Join(xdg.Home(), ".local", "share", "ryoport", "installed")
}

func readMarker() (installedMarker, error) {
	var marker installedMarker
	data, err := os.ReadFile(markerPath())
	if err != nil {
		return marker, err
	}
	if err := json.Unmarshal(data, &marker); err != nil {
		return marker, err
	}
	return marker, nil
}
