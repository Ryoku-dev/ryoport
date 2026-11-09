package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []string
}

func (runner *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, name+" "+strings.Join(args, " "))
	if len(args) >= 2 && args[0] == "-Q" && args[1] == "ryoport" {
		return []byte("ryoport 1.0.0-1\n"), nil
	}
	return nil, errors.New("unexpected command")
}

func (runner *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	runner.calls = append(runner.calls, name+" "+strings.Join(args, " "))
	return errors.New("unexpected command")
}

func TestCheckAcceptsCanonicalReleaseWithAssets(t *testing.T) {
	server := releaseServer(t, releaseSpec{tag: "v1.99.9"})
	defer server.Close()
	client := &Client{API: server.URL + "/release", HTTP: server.Client(), detect: func() (Install, string) {
		return InstallTarball, "marker"
	}}
	info, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Latest != "1.99.9" || !info.Available || !info.CanInstall || info.Install != InstallTarball {
		t.Fatalf("unexpected update info: %+v", info)
	}
}

func TestCheckRejectsInvalidAndPrereleaseTags(t *testing.T) {
	for _, test := range []releaseSpec{
		{tag: "v1.01.0"},
		{tag: "v2.0.0"},
		{tag: "v1.2.3", prerelease: true},
		{tag: "v1.2.3", draft: true},
	} {
		server := releaseServer(t, test)
		client := &Client{API: server.URL + "/release", HTTP: server.Client(), detect: func() (Install, string) {
			return InstallTarball, "marker"
		}}
		info, err := client.Check(context.Background())
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if info.Latest != "" || info.Available || info.CanInstall {
			t.Fatalf("release %+v was accepted: %+v", test, info)
		}
	}
}

func TestDistroInstallsDeferToTheirPackageManager(t *testing.T) {
	for _, manager := range []string{"dnf", "xbps", "nix"} {
		server := releaseServer(t, releaseSpec{tag: "v1.99.9"})
		runner := &fakeRunner{}
		client := &Client{
			API: server.URL + "/release", HTTP: server.Client(), runner: runner,
			detect: func() (Install, string) { return InstallDistro, manager },
		}
		info, err := client.Check(context.Background())
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		if !info.Available || info.CanInstall || !strings.Contains(info.Reason, manager) {
			server.Close()
			t.Fatalf("%s: check = %+v, want the update left to %s", manager, info, manager)
		}
		err = client.Apply(context.Background(), info, nil)
		server.Close()
		if err == nil || !strings.Contains(err.Error(), manager) {
			t.Fatalf("%s: Apply error = %v, want it left to %s", manager, err, manager)
		}
		for _, call := range runner.calls {
			if strings.Contains(call, "pkexec") || strings.Contains(call, " -U ") {
				t.Fatalf("%s: a distro install ran an installer: %s", manager, call)
			}
		}
	}
}

func TestApplyRejectsVerificationFailures(t *testing.T) {
	tests := []struct {
		name      string
		checksum  func(string, []byte) string
		oversized bool
		want      string
	}{
		{
			name: "bad digest",
			checksum: func(name string, _ []byte) string {
				return strings.Repeat("0", 64) + "  " + name + "\n"
			},
			want: "SHA-256",
		},
		{
			name: "wrong filename",
			checksum: func(_ string, body []byte) string {
				digest := sha256.Sum256(body)
				return hex.EncodeToString(digest[:]) + "  another-file.tar.gz\n"
			},
			want: "naming the release file",
		},
		{
			name: "oversize package",
			checksum: func(name string, body []byte) string {
				digest := sha256.Sum256(body)
				return hex.EncodeToString(digest[:]) + "  " + name + "\n"
			},
			oversized: true,
			want:      "too large",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := releaseServer(t, releaseSpec{tag: "v1.99.9", checksum: test.checksum, oversized: test.oversized})
			defer server.Close()
			runner := &fakeRunner{}
			client := &Client{
				API: server.URL + "/release", HTTP: server.Client(), runner: runner,
				detect: func() (Install, string) { return InstallPacman, "pacman" },
			}
			info, err := client.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = client.Apply(context.Background(), info, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Apply error = %v, want %q", err, test.want)
			}
			for _, call := range runner.calls {
				if strings.Contains(call, " -Qp ") || strings.Contains(call, "pkexec") {
					t.Fatalf("verification failure ran installer: %s", call)
				}
			}
		})
	}
}

func TestApplyInstallsTarballUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	local := filepath.Join(home, ".local")
	if err := os.MkdirAll(filepath.Join(local, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "bin", "ryoport"), []byte("old build"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := releaseServer(t, releaseSpec{tag: "v1.99.9", tarball: releaseTarball(t, "1.99.9")})
	defer server.Close()
	client := &Client{
		API: server.URL + "/release", HTTP: server.Client(), runner: stagedVersion("1.99.9"),
		detect: func() (Install, string) { return InstallTarball, markerPath() },
	}

	info, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Apply(context.Background(), info, nil); err != nil {
		t.Fatal(err)
	}

	binary, err := os.ReadFile(filepath.Join(local, "bin", "ryoport"))
	if err != nil || string(binary) != stagedScript("1.99.9") {
		t.Fatalf("bin/ryoport = %q, %v", binary, err)
	}
	if target, err := os.Readlink(filepath.Join(local, "bin", "ryovm")); err != nil || target != "ryoport" {
		t.Fatalf("bin/ryovm points at %q, %v", target, err)
	}
	marker, err := readMarker()
	if err != nil || marker.Version != "1.99.9" || !slices.Contains(marker.Files, markerFile) {
		t.Fatalf("marker = %+v, %v", marker, err)
	}
	err = filepath.WalkDir(local, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && (strings.HasSuffix(path, ".old") || strings.HasPrefix(entry.Name(), ".ryoport-install-")) {
			t.Errorf("install left %s behind", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := client.Check(context.Background())
	if err != nil || !after.RestartPending {
		t.Fatalf("after the install, check = %+v, %v; want a pending restart", after, err)
	}
}

func stagedScript(version string) string { return "#!/bin/sh\necho " + version + "\n" }

// stagedVersion answers the staged build's version query without executing it.
type stagedVersion string

func (version stagedVersion) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	if strings.HasSuffix(name, "/bin/ryoport") && len(args) == 1 && args[0] == "version" {
		return []byte(string(version) + "\n"), nil
	}
	return nil, errors.New("unexpected command")
}

func (stagedVersion) Run(context.Context, string, ...string) error {
	return errors.New("unexpected command")
}

// releaseTarball lays a release out the way scripts/build-tarball.sh does.
func releaseTarball(t *testing.T, version string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	write := func(header *tar.Header, body string) {
		header.Size = int64(len(body))
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write(&tar.Header{Name: "bin/ryoport", Mode: 0o755, Typeflag: tar.TypeReg}, stagedScript(version))
	for _, alias := range []string{"bin/ryossh", "bin/ryovm", "bin/ryovm-fetch", "bin/ryovm-mon"} {
		write(&tar.Header{Name: alias, Linkname: "ryoport", Mode: 0o777, Typeflag: tar.TypeSymlink}, "")
	}
	write(&tar.Header{Name: "share/applications/dev.ryoku.ryoport.desktop", Mode: 0o644, Typeflag: tar.TypeReg}, "[Desktop Entry]\n")
	write(&tar.Header{Name: "share/icons/hicolor/scalable/apps/dev.ryoku.ryoport.svg", Mode: 0o644, Typeflag: tar.TypeReg}, "<svg/>\n")
	write(&tar.Header{Name: "share/licenses/ryoport/LICENSE", Mode: 0o644, Typeflag: tar.TypeReg}, "GPL-3.0-or-later\n")
	write(&tar.Header{Name: "share/metainfo/dev.ryoku.ryoport.metainfo.xml", Mode: 0o644, Typeflag: tar.TypeReg}, "<component/>\n")
	write(&tar.Header{Name: markerFile, Mode: 0o644, Typeflag: tar.TypeReg}, `{"version": "`+version+`", "files": ["bin/ryoport"]}`+"\n")
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type releaseSpec struct {
	tag        string
	draft      bool
	prerelease bool
	checksum   func(string, []byte) string
	oversized  bool
	tarball    []byte
}

func releaseServer(t *testing.T, spec releaseSpec) *httptest.Server {
	t.Helper()
	packageBody := []byte("not a package")
	tarBody := packageBody
	if spec.tarball != nil {
		tarBody = spec.tarball
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		version := strings.TrimPrefix(spec.tag, "v")
		packageName := "ryoport-" + version + "-1-x86_64.pkg.tar.zst"
		tarName := "ryoport-" + version + "-linux-x86_64.tar.gz"
		body := packageBody
		if strings.HasPrefix(strings.TrimPrefix(request.URL.Path, "/"), tarName) {
			body = tarBody
		}
		switch request.URL.Path {
		case "/release":
			assets := []asset{
				{Name: packageName, Size: int64(len(packageBody)), URL: server.URL + "/" + packageName},
				{Name: packageName + ".sha256", Size: 128, URL: server.URL + "/" + packageName + ".sha256"},
				{Name: tarName, Size: int64(len(tarBody)), URL: server.URL + "/" + tarName},
				{Name: tarName + ".sha256", Size: 128, URL: server.URL + "/" + tarName + ".sha256"},
			}
			_ = json.NewEncoder(writer).Encode(release{Tag: spec.tag, Draft: spec.draft, Prerelease: spec.prerelease, Body: "notes", Assets: assets})
		case "/" + packageName, "/" + tarName:
			if spec.oversized {
				writer.Header().Set("Content-Length", fmt.Sprint(maxPackage+1))
				return
			}
			_, _ = writer.Write(body)
		case "/" + packageName + ".sha256", "/" + tarName + ".sha256":
			name := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/"), ".sha256")
			checksum := spec.checksum
			if checksum == nil {
				checksum = func(name string, body []byte) string {
					digest := sha256.Sum256(body)
					return hex.EncodeToString(digest[:]) + "  " + name + "\n"
				}
			}
			_, _ = writer.Write([]byte(checksum(name, body)))
		default:
			http.NotFound(writer, request)
		}
	}))
	return server
}
