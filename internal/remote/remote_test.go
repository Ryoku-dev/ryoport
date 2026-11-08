package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

func TestCheckApp(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ok.Close()
	if status := checkApp(context.Background(), "h", App{Name: "svc", URL: ok.URL}); status.State != "up" || status.Code != 200 {
		t.Fatalf("reachable 200: want up/200, got %s/%d", status.State, status.Code)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(503) }))
	defer bad.Close()
	if status := checkApp(context.Background(), "h", App{Name: "svc", URL: bad.URL}); status.State != "warn" || status.Code != 503 {
		t.Fatalf("answering 503: want warn/503, got %s/%d", status.State, status.Code)
	}
	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := gone.URL
	gone.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if status := checkApp(ctx, "h", App{Name: "svc", URL: url}); status.State != "down" {
		t.Fatalf("unreachable: want down, got %s", status.State)
	}
}

func TestPVEGuests(t *testing.T) {
	const body = `{"data":[
		{"vmid":200,"name":"ct","status":"running","node":"pve","type":"lxc","cpu":0.1,"mem":100,"maxmem":1000,"uptime":50},
		{"vmid":100,"name":"vm","status":"stopped","node":"pve","type":"qemu","maxmem":2000},
		{"id":"node/pve","node":"pve","type":"node"},
		{"id":"storage/local","type":"storage"}
	]}`
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth, gotPath = request.Header.Get("Authorization"), request.URL.Path
		_, _ = writer.Write([]byte(body))
	}))
	defer server.Close()
	setTestHome(t)
	if err := writeSidecar(map[string]meta{"lab": {Pve: &PVE{URL: server.URL, Token: "ryoport@pve!hub=secret"}}}); err != nil {
		t.Fatal(err)
	}
	guests, err := New().PVEGuests(context.Background(), "lab")
	if err != nil {
		t.Fatal(err)
	}
	if len(guests) != 2 || guests[0].VMID != 100 || guests[1].VMID != 200 {
		t.Fatalf("filtered guests = %+v", guests)
	}
	if guests[0].Type != "qemu" || guests[1].Type != "lxc" {
		t.Fatalf("types wrong: %s,%s", guests[0].Type, guests[1].Type)
	}
	if gotAuth != "PVEAPIToken=ryoport@pve!hub=secret" {
		t.Fatalf("auth header wrong: %q", gotAuth)
	}
	if gotPath != "/api2/json/cluster/resources" {
		t.Fatalf("path wrong: %q", gotPath)
	}
}

func TestPVEAction(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotMethod, gotPath, gotAuth = request.Method, request.URL.Path, request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"data":"UPID:pve:0"}`))
	}))
	defer server.Close()
	setTestHome(t)
	if err := writeSidecar(map[string]meta{"lab": {Pve: &PVE{URL: server.URL, Token: "u@pam!t=s"}}}); err != nil {
		t.Fatal(err)
	}
	if err := New().PVEAction(context.Background(), "lab", "pve", "qemu", 100, "start"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api2/json/nodes/pve/qemu/100/status/start" || gotAuth != "PVEAPIToken=u@pam!t=s" {
		t.Fatalf("request = %s %s %q", gotMethod, gotPath, gotAuth)
	}
}

func TestPVEValidNode(t *testing.T) {
	for _, value := range []string{"pve", "pve-1", "node.dc1", "n_2"} {
		if !validHost(value) {
			t.Fatalf("%q should be valid", value)
		}
	}
	for _, value := range []string{"", "pve/..", "a b", "n;rm", "../x"} {
		if validHost(value) {
			t.Fatalf("%q should be rejected", value)
		}
	}
}

func TestProbeErrorNeedAuth(t *testing.T) {
	cases := []struct {
		stderr      string
		usePassword bool
		wantMessage string
		wantAuth    bool
	}{
		{"Permission denied (publickey,password).", false, "authentication required", true},
		{"Too many authentication failures", false, "authentication required", true},
		{"Permission denied, please try again.", true, "saved password rejected", false},
		{"Host key verification failed.", false, "host key changed (known_hosts)", false},
		{"ssh: connect to host 10.0.0.1 port 22: No route to host", false, "ssh: connect to host 10.0.0.1 port 22: No route to host", false},
	}
	for _, test := range cases {
		message, auth := probeError(context.Background(), errors.New("exit status 255"), test.stderr, test.usePassword)
		if message != test.wantMessage || auth != test.wantAuth {
			t.Errorf("probeError(%q, %v) = (%q, %v), want (%q, %v)", test.stderr, test.usePassword, message, auth, test.wantMessage, test.wantAuth)
		}
	}
}

func TestParseTunnelSpec(t *testing.T) {
	valid := map[string][2]string{
		"L:15432:127.0.0.1:22":         {"local", "15432:127.0.0.1:22"},
		"R:localhost:8080:web.home:80": {"remote", "localhost:8080:web.home:80"},
		"D:1080":                       {"dynamic", "1080"},
		"D:127.0.0.1:1080":             {"dynamic", "127.0.0.1:1080"},
	}
	for spec, want := range valid {
		kind, arg, err := ParseTunnelSpec(spec)
		if err != nil || kind != want[0] || arg != want[1] {
			t.Errorf("ParseTunnelSpec(%q) = %q, %q, %v", spec, kind, arg, err)
		}
	}
	for _, spec := range []string{"", "X:22", "L:22:host", "L:0:host:22", "L:22:bad;host:80", "D:65536", "D:host:1:2"} {
		if _, _, err := ParseTunnelSpec(spec); err == nil {
			t.Errorf("ParseTunnelSpec(%q) accepted", spec)
		}
	}
}

func TestAliasWithPlusCanBeSavedListedAndRemoved(t *testing.T) {
	home := setTestHome(t)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	ssh := "#!/bin/sh\nprintf 'hostname mail.example\\nuser tester\\nport 22\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(ssh), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	manager := New()
	const alias = "mail+work"
	if err := manager.Save(HostInput{Alias: alias, HostName: "mail.example", User: "tester", Port: 22}); err != nil {
		t.Fatal(err)
	}
	hosts, err := manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Alias != alias {
		t.Fatalf("hosts = %#v, want alias %q", hosts, alias)
	}
	if err := manager.Remove(alias); err != nil {
		t.Fatal(err)
	}
	hosts, err = manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("hosts after remove = %#v, want none", hosts)
	}
}

func TestConnectCommandUsesResolvedTerminal(t *testing.T) {
	tests := []struct {
		name        string
		executables []string
		want        string
	}{
		{name: "kitty direct", executables: []string{"kitty"}, want: "kitten"},
		{name: "xdg terminal exec", executables: []string{"xdg-terminal-exec", "kitty"}, want: "ssh"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := setTestHome(t)
			bin := filepath.Join(home, "bin")
			if err := os.MkdirAll(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range test.executables {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			t.Setenv("TERMINAL", "")
			command := New().ConnectCommand("mail+work")
			if len(command) == 0 || command[0] != test.want {
				t.Fatalf("ConnectCommand = %v, want %s command", command, test.want)
			}
		})
	}
}

func TestTunnelWritesAndPruningDoNotLoseRecords(t *testing.T) {
	setTestHome(t)
	manager := New()
	manager.tunnelAlive = func(pid int) bool { return pid > 0 }
	if err := writeTunnels([]Tunnel{{ID: "dead", Alias: "old", Pid: 0}}); err != nil {
		t.Fatal(err)
	}

	const count = 64
	start := make(chan struct{})
	errs := make(chan error, count+1)
	var workers sync.WaitGroup
	workers.Add(count + 1)
	for i := 1; i <= count; i++ {
		go func(pid int) {
			defer workers.Done()
			<-start
			errs <- manager.recordTunnel(Tunnel{ID: fmt.Sprintf("tunnel-%d", pid), Alias: "mail+work", Pid: pid})
		}(i)
	}
	go func() {
		defer workers.Done()
		<-start
		for range count {
			_, err := manager.Tunnels("")
			if err != nil {
				errs <- err
				return
			}
		}
		errs <- nil
	}()
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	tunnels, err := manager.Tunnels("")
	if err != nil {
		t.Fatal(err)
	}
	if len(tunnels) != count {
		t.Fatalf("tunnel count = %d, want %d", len(tunnels), count)
	}
	seen := make(map[string]bool, count)
	for _, tunnel := range tunnels {
		seen[tunnel.ID] = true
	}
	for i := 1; i <= count; i++ {
		id := fmt.Sprintf("tunnel-%d", i)
		if !seen[id] {
			t.Errorf("missing %s", id)
		}
	}
}

func TestSaveRemoveManagedInclude(t *testing.T) {
	setTestHome(t)
	manager := New()
	input := HostInput{Alias: "alpha", HostName: "alpha.home", User: "nero", Port: 2222, IdentityFile: "~/.ssh/id_alpha", ProxyJump: "jump", Group: "Lab", Tags: []string{}, Notes: "note", Pinned: true, Watch: []string{"sshd.service"}}
	if err := manager.Save(input); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(xdg.SSHConfig())
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "Include config.d/*\n" {
		t.Fatalf("config = %q", config)
	}
	managed, err := os.ReadFile(xdg.SSHManaged())
	if err != nil {
		t.Fatal(err)
	}
	wantManaged := "# Managed by ryoport. Do not edit; use the ryoport app or `ryossh add`/`ryossh remove`.\n\nHost alpha\n    HostName alpha.home\n    User nero\n    Port 2222\n    IdentityFile ~/.ssh/id_alpha\n    ProxyJump jump\n\n"
	if string(managed) != wantManaged {
		t.Fatalf("managed include:\n%s\nwant:\n%s", managed, wantManaged)
	}
	remoteBytes, err := os.ReadFile(xdg.RemotesFile())
	if err != nil {
		t.Fatal(err)
	}
	wantSidecar := "{\n  \"alpha\": {\n    \"group\": \"Lab\",\n    \"tags\": [],\n    \"notes\": \"note\",\n    \"pinned\": true,\n    \"watch\": [\n      \"sshd.service\"\n    ],\n    \"apps\": null\n  }\n}\n"
	if string(remoteBytes) != wantSidecar {
		t.Fatalf("remotes.json:\n%s\nwant:\n%s", remoteBytes, wantSidecar)
	}
	if err := manager.Remove("alpha"); err != nil {
		t.Fatal(err)
	}
	managed, _ = os.ReadFile(xdg.SSHManaged())
	if string(managed) != "# Managed by ryoport. Do not edit; use the ryoport app or `ryossh add`/`ryossh remove`.\n\n" {
		t.Fatalf("managed after remove = %q", managed)
	}
	remoteBytes, _ = os.ReadFile(xdg.RemotesFile())
	if string(remoteBytes) != "{}\n" {
		t.Fatalf("remotes after remove = %q", remoteBytes)
	}
}

func TestProbeJSONShapes(t *testing.T) {
	success, err := json.Marshal(Probe{Alias: "h", OK: true, TopProcs: []string{}, Services: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"cpuPercent":0`, `"memTotalKb":0`, `"topProcs":[]`, `"services":{}`} {
		if !strings.Contains(string(success), field) {
			t.Errorf("success JSON %s lacks %s", success, field)
		}
	}
	failure, err := json.Marshal(Probe{Alias: "h", Error: "down"})
	if err != nil {
		t.Fatal(err)
	}
	if string(failure) != `{"alias":"h","ok":false,"error":"down","needAuth":false}` {
		t.Fatalf("failure JSON = %s", failure)
	}
}

func setTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}
