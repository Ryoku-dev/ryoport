package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type releaseSpec struct {
	tag        string
	draft      bool
	prerelease bool
	checksum   func(string, []byte) string
	oversized  bool
}

func releaseServer(t *testing.T, spec releaseSpec) *httptest.Server {
	t.Helper()
	packageBody := []byte("not a package")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		version := strings.TrimPrefix(spec.tag, "v")
		packageName := "ryoport-" + version + "-1-x86_64.pkg.tar.zst"
		tarName := "ryoport-" + version + "-linux-x86_64.tar.gz"
		switch request.URL.Path {
		case "/release":
			assets := []asset{
				{Name: packageName, Size: int64(len(packageBody)), URL: server.URL + "/" + packageName},
				{Name: packageName + ".sha256", Size: 128, URL: server.URL + "/" + packageName + ".sha256"},
				{Name: tarName, Size: int64(len(packageBody)), URL: server.URL + "/" + tarName},
				{Name: tarName + ".sha256", Size: 128, URL: server.URL + "/" + tarName + ".sha256"},
			}
			_ = json.NewEncoder(writer).Encode(release{Tag: spec.tag, Draft: spec.draft, Prerelease: spec.prerelease, Body: "notes", Assets: assets})
		case "/" + packageName, "/" + tarName:
			if spec.oversized {
				writer.Header().Set("Content-Length", fmt.Sprint(maxPackage+1))
				return
			}
			_, _ = writer.Write(packageBody)
		case "/" + packageName + ".sha256", "/" + tarName + ".sha256":
			name := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/"), ".sha256")
			checksum := spec.checksum
			if checksum == nil {
				checksum = func(name string, body []byte) string {
					digest := sha256.Sum256(body)
					return hex.EncodeToString(digest[:]) + "  " + name + "\n"
				}
			}
			_, _ = writer.Write([]byte(checksum(name, packageBody)))
		default:
			http.NotFound(writer, request)
		}
	}))
	return server
}
