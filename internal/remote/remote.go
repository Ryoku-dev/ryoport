// Package remote manages SSH servers, probes, keys and tunnels.
package remote

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/keyring"
	"github.com/ryoku-dev/ryoport/internal/launch"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const (
	poolSize  = 8
	secretKey = "ryoport-alias"
	sshClass  = "dev.ryoku.ryoport_ssh"
)

type App struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PVE struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
}

// Host is an SSH config entry merged with its Ryoport metadata.
type Host struct {
	Alias        string   `json:"alias"`
	HostName     string   `json:"hostName"`
	User         string   `json:"user"`
	Port         int      `json:"port"`
	IdentityFile string   `json:"identityFile"`
	ProxyJump    string   `json:"proxyJump"`
	Group        string   `json:"group"`
	Tags         []string `json:"tags"`
	Notes        string   `json:"notes"`
	Pinned       bool     `json:"pinned"`
	Watch        []string `json:"watch"`
	Apps         []App    `json:"apps"`
	Pve          *PVE     `json:"pve,omitempty"`
	Auth         string   `json:"auth,omitempty"`
}

// HostInput is the host shape accepted by Save.
type HostInput struct {
	Alias        string   `json:"alias"`
	HostName     string   `json:"hostName"`
	User         string   `json:"user"`
	Port         int      `json:"port"`
	IdentityFile string   `json:"identityFile"`
	ProxyJump    string   `json:"proxyJump"`
	Group        string   `json:"group"`
	Tags         []string `json:"tags"`
	Notes        string   `json:"notes"`
	Pinned       bool     `json:"pinned"`
	Watch        []string `json:"watch"`
	Apps         []App    `json:"apps"`
	Pve          *PVE     `json:"pve,omitempty"`
	Auth         string   `json:"auth,omitempty"`
}

type Reach struct {
	Alias string `json:"alias"`
	Up    bool   `json:"up"`
	RTTMs int    `json:"rttMs"`
	SSHUp bool   `json:"sshUp"`
}

type Probe struct {
	Alias          string            `json:"alias"`
	OK             bool              `json:"ok"`
	Host           string            `json:"host,omitempty"`
	Kernel         string            `json:"kernel,omitempty"`
	Distro         string            `json:"distro,omitempty"`
	UptimeS        int64             `json:"uptimeS,omitempty"`
	CPUPercent     float64           `json:"cpuPercent,omitempty"`
	Load1          float64           `json:"load1,omitempty"`
	Load5          float64           `json:"load5,omitempty"`
	Load15         float64           `json:"load15,omitempty"`
	CPUs           int               `json:"cpus,omitempty"`
	MemTotalKb     int64             `json:"memTotalKb,omitempty"`
	MemAvailKb     int64             `json:"memAvailKb,omitempty"`
	DiskTotalKb    int64             `json:"diskTotalKb,omitempty"`
	DiskUsedKb     int64             `json:"diskUsedKb,omitempty"`
	DiskPct        int               `json:"diskPct,omitempty"`
	DiskReadBytes  int64             `json:"diskReadBytes,omitempty"`
	DiskWriteBytes int64             `json:"diskWriteBytes,omitempty"`
	NetRxBytes     int64             `json:"netRxBytes,omitempty"`
	NetTxBytes     int64             `json:"netTxBytes,omitempty"`
	FailedUnits    int               `json:"failedUnits,omitempty"`
	Logins         int               `json:"logins,omitempty"`
	TopProcs       []string          `json:"topProcs,omitempty"`
	Services       map[string]string `json:"services,omitempty"`
	Error          string            `json:"error,omitempty"`
	NeedAuth       bool              `json:"needAuth,omitempty"`
}

func (p Probe) MarshalJSON() ([]byte, error) {
	if !p.OK {
		return json.Marshal(struct {
			Alias    string `json:"alias"`
			OK       bool   `json:"ok"`
			Error    string `json:"error"`
			NeedAuth bool   `json:"needAuth"`
		}{Alias: p.Alias, Error: p.Error, NeedAuth: p.NeedAuth})
	}
	return json.Marshal(struct {
		Alias          string            `json:"alias"`
		OK             bool              `json:"ok"`
		Host           string            `json:"host"`
		Kernel         string            `json:"kernel"`
		Distro         string            `json:"distro"`
		UptimeS        int64             `json:"uptimeS"`
		CPUPercent     float64           `json:"cpuPercent"`
		Load1          float64           `json:"load1"`
		Load5          float64           `json:"load5"`
		Load15         float64           `json:"load15"`
		CPUs           int               `json:"cpus"`
		MemTotalKb     int64             `json:"memTotalKb"`
		MemAvailKb     int64             `json:"memAvailKb"`
		DiskTotalKb    int64             `json:"diskTotalKb"`
		DiskUsedKb     int64             `json:"diskUsedKb"`
		DiskPct        int               `json:"diskPct"`
		DiskReadBytes  int64             `json:"diskReadBytes"`
		DiskWriteBytes int64             `json:"diskWriteBytes"`
		NetRxBytes     int64             `json:"netRxBytes"`
		NetTxBytes     int64             `json:"netTxBytes"`
		FailedUnits    int               `json:"failedUnits"`
		Logins         int               `json:"logins"`
		TopProcs       []string          `json:"topProcs"`
		Services       map[string]string `json:"services"`
	}{
		Alias: p.Alias, OK: true, Host: p.Host, Kernel: p.Kernel, Distro: p.Distro,
		UptimeS: p.UptimeS, CPUPercent: p.CPUPercent, Load1: p.Load1, Load5: p.Load5,
		Load15: p.Load15, CPUs: p.CPUs, MemTotalKb: p.MemTotalKb, MemAvailKb: p.MemAvailKb,
		DiskTotalKb: p.DiskTotalKb, DiskUsedKb: p.DiskUsedKb, DiskPct: p.DiskPct,
		DiskReadBytes: p.DiskReadBytes, DiskWriteBytes: p.DiskWriteBytes,
		NetRxBytes: p.NetRxBytes, NetTxBytes: p.NetTxBytes, FailedUnits: p.FailedUnits,
		Logins: p.Logins, TopProcs: p.TopProcs, Services: p.Services,
	})
}

type AppCheck struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	State string `json:"state"`
	Ms    int64  `json:"ms"`
	Code  int    `json:"code"`
}

type Guest struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	Type   string  `json:"type"`
	CPU    float64 `json:"cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

type Tunnel struct {
	ID     string `json:"id"`
	Alias  string `json:"alias"`
	Kind   string `json:"kind"`
	Spec   string `json:"spec"`
	Pid    int    `json:"pid"`
	Opened int64  `json:"opened"`
}

type AgentKey struct {
	Bits        int    `json:"bits"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	Type        string `json:"type"`
}

type FileKey struct {
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	Comment     string `json:"comment"`
}

type Keys struct {
	Agent []AgentKey `json:"agent"`
	Files []FileKey  `json:"files"`
}

type meta struct {
	Group  string   `json:"group"`
	Tags   []string `json:"tags"`
	Notes  string   `json:"notes"`
	Pinned bool     `json:"pinned"`
	Watch  []string `json:"watch"`
	Apps   []App    `json:"apps"`
	Pve    *PVE     `json:"pve,omitempty"`
	Auth   string   `json:"auth,omitempty"`
}

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) List(ctx context.Context) ([]Host, error) {
	aliases := listAliases()
	side := readSidecar()
	hosts := make([]Host, len(aliases))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, alias := range aliases {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			hosts[i] = buildHost(ctx, alias, side)
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Group != hosts[j].Group {
			return hosts[i].Group < hosts[j].Group
		}
		return hosts[i].Alias < hosts[j].Alias
	})
	return hosts, nil
}

func (m *Manager) Ping(ctx context.Context, alias string) Reach {
	if !ValidAlias(alias) {
		return Reach{Alias: alias, RTTMs: -1}
	}
	cfg := resolveConfig(ctx, alias)
	host, port := resolveHostPort(cfg)
	result := Reach{Alias: alias, RTTMs: -1}
	if host == "" {
		return result
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return result
	}
	conn.Close()
	result.Up = true
	result.SSHUp = true
	result.RTTMs = int(time.Since(start).Milliseconds())
	return result
}

func (m *Manager) PingAll(ctx context.Context) []Reach {
	aliases := listAliases()
	results := make([]Reach, len(aliases))
	parallel(ctx, len(aliases), func(i int) { results[i] = m.Ping(ctx, aliases[i]) })
	return results
}

func (m *Manager) Probe(ctx context.Context, alias string) Probe {
	if !ValidAlias(alias) {
		return Probe{Alias: alias, Error: "bad alias"}
	}
	entry := readSidecar()[alias]
	probeCtx, cancel := withDefaultTimeout(ctx, 15*time.Second)
	defer cancel()
	return probeOne(probeCtx, alias, entry.Watch, entry.Auth == "password")
}

func (m *Manager) ProbeAll(ctx context.Context) []Probe {
	aliases := listAliases()
	side := readSidecar()
	results := make([]Probe, len(aliases))
	parallel(ctx, len(aliases), func(i int) {
		entry := side[aliases[i]]
		probeCtx, cancel := withDefaultTimeout(ctx, 8*time.Second)
		defer cancel()
		results[i] = probeOne(probeCtx, aliases[i], entry.Watch, entry.Auth == "password")
	})
	return results
}

func (m *Manager) AppCheck(ctx context.Context, alias string) []AppCheck {
	if !ValidAlias(alias) {
		return []AppCheck{}
	}
	return checkApps(ctx, alias, readSidecar()[alias].Apps)
}

func (m *Manager) AppCheckAll(ctx context.Context) []AppCheck {
	side := readSidecar()
	aliases := make([]string, 0, len(side))
	for alias := range side {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	all := []AppCheck{}
	for _, alias := range aliases {
		all = append(all, checkApps(ctx, alias, side[alias].Apps)...)
	}
	return all
}

func (m *Manager) PVEGuests(ctx context.Context, alias string) ([]Guest, error) {
	pve, err := pveOf(alias)
	if err != nil {
		return nil, err
	}
	body, err := pveDo(ctx, pve, http.MethodGet, "/api2/json/cluster/resources")
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []Guest `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	guests := []Guest{}
	for _, guest := range response.Data {
		if guest.Type == "qemu" || guest.Type == "lxc" {
			guests = append(guests, guest)
		}
	}
	sort.Slice(guests, func(i, j int) bool { return guests[i].VMID < guests[j].VMID })
	return guests, nil
}

func (m *Manager) PVEAction(ctx context.Context, alias, node, kind string, vmid int, action string) error {
	if !validHost(node) {
		return errors.New("bad node name")
	}
	if kind != "qemu" && kind != "lxc" {
		return errors.New("type must be qemu or lxc")
	}
	if vmid < 1 {
		return errors.New("vmid must be positive")
	}
	switch action {
	case "start", "stop", "shutdown", "reboot":
	default:
		return errors.New("action must be start, stop, shutdown or reboot")
	}
	pve, err := pveOf(alias)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api2/json/nodes/%s/%s/%d/status/%s", node, kind, vmid, action)
	_, err = pveDo(ctx, pve, http.MethodPost, path)
	return err
}

func (m *Manager) Connect(alias string) error {
	command := m.ConnectCommand(alias)
	if len(command) == 0 {
		return errors.New("bad alias")
	}
	command, err := m.withPasswordEnv(alias, command)
	if err != nil {
		return err
	}
	return launch.Terminal(command, sshClass, "ssh: "+alias)
}

func (m *Manager) ConnectCommand(alias string) []string {
	if !ValidAlias(alias) {
		return nil
	}
	opts := []string{"-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", alias}
	term := os.Getenv("TERMINAL")
	if term == "" {
		if _, err := exec.LookPath("xdg-terminal-exec"); err != nil {
			if _, err := exec.LookPath("kitty"); err == nil {
				term = "kitty"
			}
		}
	}
	if filepath.Base(term) == "kitty" && readSidecar()[alias].Auth != "password" {
		return append([]string{"kitten", "ssh"}, opts...)
	}
	return append([]string{"ssh"}, opts...)
}

func (m *Manager) RunInTerminal(alias, command string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	if command == "" || strings.ContainsAny(command, "\x00\r\n") {
		return errors.New("invalid remote command")
	}
	argv := []string{"ssh", "-t", alias, command}
	argv, err := m.withPasswordEnv(alias, argv)
	if err != nil {
		return err
	}
	return launch.Terminal(argv, sshClass, "ssh: "+alias)
}

func (m *Manager) SetPassword(alias, password string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	if password == "" {
		return errors.New("no password")
	}
	if !keyring.Available() {
		return errors.New("secret-tool not found: install libsecret to save passwords")
	}
	if err := keyring.Store("ryoport: "+alias, map[string]string{secretKey: alias}, password); err != nil {
		return err
	}
	side := readSidecar()
	entry := side[alias]
	entry.Auth = "password"
	side[alias] = entry
	return writeSidecar(side)
}

func (m *Manager) ClearPassword(alias string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	if keyring.Available() {
		_ = keyring.Clear(map[string]string{secretKey: alias})
	}
	side := readSidecar()
	if entry, ok := side[alias]; ok {
		entry.Auth = ""
		side[alias] = entry
		return writeSidecar(side)
	}
	return nil
}

func (m *Manager) Keys(ctx context.Context) (Keys, error) {
	result := Keys{Agent: []AgentKey{}, Files: []FileKey{}}
	if output, err := exec.CommandContext(ctx, "ssh-add", "-l").Output(); err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if bits, fingerprint, kind, comment, ok := parseKeyLine(line); ok {
				result.Agent = append(result.Agent, AgentKey{Bits: bits, Fingerprint: fingerprint, Comment: comment, Type: kind})
			}
		}
	}
	pubs, _ := filepath.Glob(filepath.Join(xdg.SSHDir(), "*.pub"))
	sort.Strings(pubs)
	for _, pub := range pubs {
		output, err := exec.CommandContext(ctx, "ssh-keygen", "-lf", pub).Output()
		if err != nil {
			continue
		}
		if _, fingerprint, kind, comment, ok := parseKeyLine(strings.TrimRight(string(output), "\n")); ok {
			result.Files = append(result.Files, FileKey{Path: shortenHome(pub), Fingerprint: fingerprint, Type: kind, Comment: comment})
		}
	}
	return result, ctx.Err()
}

func (m *Manager) CopyID(alias, publicKey string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	publicKey = expandHome(publicKey)
	if publicKey == "" {
		if identity := resolveConfig(context.Background(), alias)["identityfile"]; identity != "" {
			candidate := expandHome(identity) + ".pub"
			if _, err := os.Stat(candidate); err == nil {
				publicKey = candidate
			}
		}
	}
	args := []string{"ssh-copy-id"}
	if publicKey != "" {
		if strings.ContainsAny(publicKey, "\x00\r\n") {
			return errors.New("invalid public key path")
		}
		args = append(args, "-i", publicKey)
	}
	args = append(args, alias)
	args, err := m.withPasswordEnv(alias, args)
	if err != nil {
		return err
	}
	return launch.Terminal(args, sshClass, "copy key: "+alias)
}

func (m *Manager) Keygen(name, kind string) (string, error) {
	if !validKeyName(name) {
		return "", errors.New("invalid key name")
	}
	if kind == "" {
		kind = "ed25519"
	}
	if !validWord(kind) {
		return "", errors.New("invalid key type")
	}
	if err := xdg.EnsureDir(xdg.SSHDir()); err != nil {
		return "", err
	}
	key := filepath.Join(xdg.SSHDir(), name)
	if _, err := os.Stat(key); err == nil {
		return "", fmt.Errorf("key already exists: %s", shortenHome(key))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	output, err := exec.Command("ssh-keygen", "-t", kind, "-f", key, "-C", name+"@ryoku", "-N", "").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return shortenHome(key + ".pub"), nil
}

func (m *Manager) KnownRemove(alias string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	host, port := resolveHostPort(resolveConfig(context.Background(), alias))
	if host == "" {
		host = alias
	}
	target := host
	if port != 22 {
		target = fmt.Sprintf("[%s]:%d", host, port)
	}
	output, err := exec.Command("ssh-keygen", "-R", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen -R: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *Manager) Save(input HostInput) error {
	if !ValidAlias(input.Alias) {
		return errors.New("invalid alias")
	}
	if input.HostName == "" || strings.ContainsAny(input.HostName, " \t\r\n") || strings.HasPrefix(input.HostName, "-") {
		return errors.New("invalid host name")
	}
	for _, value := range []string{input.User, input.IdentityFile, input.ProxyJump} {
		if strings.ContainsAny(value, "\r\n") || strings.HasPrefix(value, "-") {
			return errors.New("host fields must not contain newlines or start with '-'")
		}
	}
	if input.Port < 0 || input.Port > 65535 {
		return errors.New("port must be 1-65535")
	}
	if err := xdg.EnsureDir(xdg.SSHDir()); err != nil {
		return err
	}
	if err := ensureIncludeLine(); err != nil {
		return err
	}
	header, blocks := loadInclude(xdg.SSHManaged())
	block := includeBlock{alias: input.Alias, lines: stanzaLines(input)}
	replaced := false
	for i := range blocks {
		if blocks[i].alias == input.Alias {
			blocks[i] = block
			replaced = true
			break
		}
	}
	if !replaced {
		blocks = append(blocks, block)
	}
	if err := writeInclude(xdg.SSHManaged(), header, blocks); err != nil {
		return err
	}
	side := readSidecar()
	previous := side[input.Alias]
	side[input.Alias] = meta{
		Group: input.Group, Tags: nonNil(input.Tags), Notes: input.Notes, Pinned: input.Pinned,
		Watch: nonNil(input.Watch), Apps: input.Apps, Pve: input.Pve, Auth: previous.Auth,
	}
	return writeSidecar(side)
}

func (m *Manager) Remove(alias string) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	header, blocks := loadInclude(xdg.SSHManaged())
	kept := blocks[:0]
	for _, block := range blocks {
		if block.alias != alias {
			kept = append(kept, block)
		}
	}
	if len(kept) != len(blocks) {
		if err := writeInclude(xdg.SSHManaged(), header, kept); err != nil {
			return err
		}
	}
	if keyring.Available() {
		_ = keyring.Clear(map[string]string{secretKey: alias})
	}
	side := readSidecar()
	if _, ok := side[alias]; ok {
		delete(side, alias)
		return writeSidecar(side)
	}
	return nil
}

func (m *Manager) SetPinned(alias string, pinned bool) error {
	if !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	side := readSidecar()
	entry := side[alias]
	entry.Pinned = pinned
	side[alias] = entry
	return writeSidecar(side)
}

func (m *Manager) Tunnels(alias string) ([]Tunnel, error) {
	if alias != "" && !ValidAlias(alias) {
		return nil, errors.New("bad alias")
	}
	original := readTunnels()
	live := pruneTunnels(original)
	if len(live) != len(original) {
		if err := writeTunnels(live); err != nil {
			return nil, err
		}
	}
	result := make([]Tunnel, 0, len(live))
	for _, tunnel := range live {
		if alias == "" || tunnel.Alias == alias {
			result = append(result, tunnel)
		}
	}
	return result, nil
}

func (m *Manager) OpenTunnel(alias, spec string) (Tunnel, error) {
	if !ValidAlias(alias) {
		return Tunnel{}, errors.New("bad alias")
	}
	kind, arg, err := ParseTunnelSpec(spec)
	if err != nil {
		return Tunnel{}, err
	}
	flag := map[string]string{"local": "-L", "remote": "-R", "dynamic": "-D"}[kind]
	argv := []string{"-N", "-o", "ExitOnForwardFailure=yes", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", "-o", "BatchMode=yes", flag, arg, alias}
	cmd := exec.Command("ssh", argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return Tunnel{}, err
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	cmd.Stdout = devnull
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Tunnel{}, fmt.Errorf("spawn ssh: %w", err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case waitErr := <-done:
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = fmt.Sprintf("ssh exited early: %v", waitErr)
		}
		return Tunnel{}, errors.New(message)
	case <-time.After(800 * time.Millisecond):
	}
	id, err := randomID()
	if err != nil {
		killGroup(pid)
		return Tunnel{}, err
	}
	tunnel := Tunnel{ID: id, Alias: alias, Kind: kind, Spec: spec, Pid: pid, Opened: time.Now().Unix()}
	if err := writeTunnels(append(pruneTunnels(readTunnels()), tunnel)); err != nil {
		killGroup(pid)
		return Tunnel{}, err
	}
	return tunnel, nil
}

func (m *Manager) CloseTunnel(id string) error {
	if id == "" || strings.ContainsAny(id, "\x00\r\n") {
		return errors.New("invalid tunnel id")
	}
	tunnels := readTunnels()
	index := -1
	for i, tunnel := range tunnels {
		if tunnel.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("no such tunnel")
	}
	killGroup(tunnels[index].Pid)
	kept := append(tunnels[:index:index], tunnels[index+1:]...)
	return writeTunnels(pruneTunnels(kept))
}

func (m *Manager) CloseAll(alias string) error {
	if alias != "" && !ValidAlias(alias) {
		return errors.New("bad alias")
	}
	tunnels := readTunnels()
	kept := make([]Tunnel, 0, len(tunnels))
	for _, tunnel := range tunnels {
		if alias == "" || tunnel.Alias == alias {
			killGroup(tunnel.Pid)
			continue
		}
		kept = append(kept, tunnel)
	}
	return writeTunnels(pruneTunnels(kept))
}

func ValidAlias(value string) bool {
	return validHost(value) && !strings.HasPrefix(value, "-")
}

// ParseTunnelSpec validates a forward and returns its kind and ssh argument.
func ParseTunnelSpec(spec string) (kind, arg string, err error) {
	if len(spec) < 3 || spec[1] != ':' {
		return "", "", fmt.Errorf("bad spec %q: must start with L:, R:, or D:", spec)
	}
	arg = spec[2:]
	parts := strings.Split(arg, ":")
	checkPort := func(value string) error {
		port, parseErr := strconv.Atoi(value)
		if parseErr != nil || port < 1 || port > 65535 {
			return fmt.Errorf("bad spec %q: port %q must be 1-65535", spec, value)
		}
		return nil
	}
	checkHost := func(value string) error {
		if !validHost(value) {
			return fmt.Errorf("bad spec %q: host %q has invalid characters", spec, value)
		}
		return nil
	}
	switch spec[0] {
	case 'L', 'R':
		var bind, listen, host, destination string
		switch len(parts) {
		case 3:
			listen, host, destination = parts[0], parts[1], parts[2]
		case 4:
			bind, listen, host, destination = parts[0], parts[1], parts[2], parts[3]
		default:
			return "", "", fmt.Errorf("bad spec %q: expected [bind:]port:host:port", spec)
		}
		if bind != "" {
			if err := checkHost(bind); err != nil {
				return "", "", err
			}
		}
		if err := checkPort(listen); err != nil {
			return "", "", err
		}
		if err := checkHost(host); err != nil {
			return "", "", err
		}
		if err := checkPort(destination); err != nil {
			return "", "", err
		}
		if spec[0] == 'L' {
			return "local", arg, nil
		}
		return "remote", arg, nil
	case 'D':
		var bind, port string
		switch len(parts) {
		case 1:
			port = parts[0]
		case 2:
			bind, port = parts[0], parts[1]
		default:
			return "", "", fmt.Errorf("bad spec %q: expected [bind:]port", spec)
		}
		if bind != "" {
			if err := checkHost(bind); err != nil {
				return "", "", err
			}
		}
		if err := checkPort(port); err != nil {
			return "", "", err
		}
		return "dynamic", arg, nil
	default:
		return "", "", fmt.Errorf("bad spec %q: must start with L:, R:, or D:", spec)
	}
}

func (m *Manager) withPasswordEnv(alias string, argv []string) ([]string, error) {
	if readSidecar()[alias].Auth != "password" {
		return argv, nil
	}
	askpass, err := ensureAskpass()
	if err != nil {
		return nil, err
	}
	return append([]string{"env", "SSH_ASKPASS=" + askpass, "SSH_ASKPASS_REQUIRE=force", "RYOPORT_ALIAS=" + alias}, argv...), nil
}

func resolveConfig(ctx context.Context, alias string) map[string]string {
	values := map[string]string{}
	if !ValidAlias(alias) {
		return values
	}
	output, err := exec.CommandContext(ctx, "ssh", "-G", alias).Output()
	if err != nil {
		return values
	}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		key = strings.ToLower(key)
		if _, exists := values[key]; !exists {
			values[key] = value
		}
	}
	return values
}

func resolveHostPort(config map[string]string) (string, int) {
	port := 22
	if value, err := strconv.Atoi(config["port"]); err == nil && value > 0 {
		port = value
	}
	return config["hostname"], port
}

func buildHost(ctx context.Context, alias string, side map[string]meta) Host {
	config := resolveConfig(ctx, alias)
	host := Host{Alias: alias, HostName: config["hostname"], User: config["user"], IdentityFile: config["identityfile"], Tags: []string{}, Watch: []string{}, Apps: []App{}}
	_, host.Port = resolveHostPort(config)
	if jump := config["proxyjump"]; jump != "" && jump != "none" {
		host.ProxyJump = jump
	}
	if entry, ok := side[alias]; ok {
		host.Group, host.Notes, host.Pinned, host.Pve, host.Auth = entry.Group, entry.Notes, entry.Pinned, entry.Pve, entry.Auth
		if entry.Tags != nil {
			host.Tags = entry.Tags
		}
		if entry.Watch != nil {
			host.Watch = entry.Watch
		}
		if entry.Apps != nil {
			host.Apps = entry.Apps
		}
	}
	return host
}

func listAliases() []string {
	aliases := []string{}
	seen := map[string]bool{}
	visited := map[string]bool{}
	var scan func(string)
	scan = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			keyword, args := splitKV(line)
			switch strings.ToLower(keyword) {
			case "host":
				for _, pattern := range strings.Fields(args) {
					if strings.ContainsAny(pattern, "*?!") || seen[pattern] {
						continue
					}
					seen[pattern] = true
					aliases = append(aliases, pattern)
				}
			case "include":
				for _, include := range strings.Fields(args) {
					for _, match := range expandInclude(include) {
						scan(match)
					}
				}
			}
		}
	}
	scan(xdg.SSHConfig())
	return aliases
}

func expandInclude(include string) []string {
	path := expandHome(include)
	if !filepath.IsAbs(path) {
		path = filepath.Join(xdg.SSHDir(), path)
	}
	matches, err := filepath.Glob(path)
	if err != nil {
		return nil
	}
	kept := matches[:0]
	for _, match := range matches {
		if !strings.HasPrefix(filepath.Base(match), ".") {
			kept = append(kept, match)
		}
	}
	return kept
}

const baseProbeScript = `LC_ALL=C
echo "ok=1"
echo "host=$(hostname 2>/dev/null)"
echo "kernel=$(uname -r)"
[ -r /etc/os-release ] && . /etc/os-release && echo "distro=$PRETTY_NAME"
read u _ < /proc/uptime; echo "uptime_s=${u%.*}"
read _ u1 n1 s1 i1 w1 q1 sq1 st1 _ < /proc/stat
t1=$((u1+n1+s1+i1+w1+q1+sq1+st1)); idle1=$((i1+w1))
sleep 1
read _ u2 n2 s2 i2 w2 q2 sq2 st2 _ < /proc/stat
t2=$((u2+n2+s2+i2+w2+q2+sq2+st2)); idle2=$((i2+w2))
dt=$((t2-t1)); di=$((idle2-idle1))
awk -v dt="$dt" -v di="$di" "BEGIN{if(dt>0) printf \"cpu_pct=%.1f\n\",100*(dt-di)/dt; else print \"cpu_pct=0\"}"
read l1 l5 l15 _ < /proc/loadavg; echo "load1=$l1"; echo "load5=$l5"; echo "load15=$l15"
echo "cpus=$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo)"
awk "/^MemTotal:/{t=\$2}/^MemAvailable:/{a=\$2}END{print \"mem_total_kb=\"t; print \"mem_avail_kb=\"a}" /proc/meminfo
df -P -k / | awk "NR==2{print \"disk_total_kb=\"\$2; print \"disk_used_kb=\"\$3; print \"disk_avail_kb=\"\$4; print \"disk_pct=\"\$5}"
set -- $(awk '{r+=$3; w+=$7} END{print r*512, w*512}' /sys/block/*/stat 2>/dev/null)
echo "disk_read_bytes=${1:-0}"; echo "disk_write_bytes=${2:-0}"
set -- $(awk -F'[: ]+' 'NR>2 && $2!="lo"{rx+=$3; tx+=$11} END{print rx+0, tx+0}' /proc/net/dev)
echo "net_rx_bytes=${1:-0}"; echo "net_tx_bytes=${2:-0}"
command -v systemctl >/dev/null 2>&1 && echo "failed_units=$(systemctl --failed --no-legend 2>/dev/null | wc -l)"
echo "logins=$(who 2>/dev/null | wc -l)"
ps -eo pcpu,comm --sort=-pcpu 2>/dev/null | awk "NR>1&&NR<=4{printf \"top%d=%s:%s\n\",NR-1,\$2,\$1}"`

func probeOne(ctx context.Context, alias string, watch []string, usePassword bool) Probe {
	_ = xdg.EnsureDir(xdg.SSHDir())
	opts := nonInteractiveOpts()
	env := os.Environ()
	if usePassword {
		opts = passwordOpts()
		askpass, err := ensureAskpass()
		if err != nil {
			return Probe{Alias: alias, Error: err.Error()}
		}
		env = append(env, "SSH_ASKPASS="+askpass, "SSH_ASKPASS_REQUIRE=force", "RYOPORT_ALIAS="+alias)
	}
	args := append(opts, alias, "sh", "-s")
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(buildProbeScript(watch))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	values := parseKV(string(output))
	if err != nil || values["ok"] != "1" {
		message, auth := probeError(ctx, err, stderr.String(), usePassword)
		return Probe{Alias: alias, Error: message, NeedAuth: auth}
	}
	return fillProbe(alias, values)
}

func nonInteractiveOpts() []string {
	return []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=6", "-o", "StrictHostKeyChecking=accept-new", "-o", "ControlMaster=auto", "-o", "ControlPath=" + xdg.SSHControlPath(), "-o", "ControlPersist=60"}
}

func passwordOpts() []string {
	return []string{"-o", "ConnectTimeout=6", "-o", "StrictHostKeyChecking=accept-new", "-o", "ControlMaster=auto", "-o", "ControlPath=" + xdg.SSHControlPath(), "-o", "ControlPersist=60", "-o", "NumberOfPasswordPrompts=1", "-o", "PreferredAuthentications=publickey,password"}
}

func buildProbeScript(watch []string) string {
	var script strings.Builder
	script.WriteString(baseProbeScript)
	for _, service := range watch {
		if validUnit(service) {
			script.WriteString("\necho \"service." + service + "=$(systemctl is-active " + service + " 2>/dev/null || echo unknown)\"")
		}
	}
	return script.String()
}

func probeError(ctx context.Context, commandErr error, stderr string, usePassword bool) (string, bool) {
	message := strings.TrimSpace(stderr)
	if index := strings.LastIndexByte(message, '\n'); index >= 0 {
		message = message[index+1:]
	}
	lower := strings.ToLower(message)
	authBlocked := strings.Contains(lower, "permission denied") || strings.Contains(lower, "too many authentication failures") || strings.Contains(lower, "authentications that can continue") || strings.Contains(lower, "no supported authentication methods")
	if authBlocked {
		if usePassword {
			return "saved password rejected", false
		}
		return "authentication required", true
	}
	if strings.Contains(lower, "host key verification failed") {
		return "host key changed (known_hosts)", false
	}
	if strings.Contains(lower, "unable to negotiate") {
		return "ssh handshake failed (very old server?)", false
	}
	if message != "" {
		return message, false
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timed out", false
	}
	if commandErr != nil {
		return commandErr.Error(), false
	}
	return "unreachable", false
}

func fillProbe(alias string, values map[string]string) Probe {
	probe := Probe{Alias: alias, OK: true, Host: values["host"], Kernel: values["kernel"], Distro: values["distro"], UptimeS: parseInt64(values["uptime_s"]), CPUPercent: parseFloat(values["cpu_pct"]), Load1: parseFloat(values["load1"]), Load5: parseFloat(values["load5"]), Load15: parseFloat(values["load15"]), CPUs: parseInt(values["cpus"]), MemTotalKb: parseInt64(values["mem_total_kb"]), MemAvailKb: parseInt64(values["mem_avail_kb"]), DiskTotalKb: parseInt64(values["disk_total_kb"]), DiskUsedKb: parseInt64(values["disk_used_kb"]), DiskPct: parseInt(strings.TrimSuffix(values["disk_pct"], "%")), DiskReadBytes: parseInt64(values["disk_read_bytes"]), DiskWriteBytes: parseInt64(values["disk_write_bytes"]), NetRxBytes: parseInt64(values["net_rx_bytes"]), NetTxBytes: parseInt64(values["net_tx_bytes"]), FailedUnits: parseInt(values["failed_units"]), Logins: parseInt(values["logins"]), TopProcs: []string{}, Services: map[string]string{}}
	for i := 1; ; i++ {
		value, ok := values["top"+strconv.Itoa(i)]
		if !ok {
			break
		}
		probe.TopProcs = append(probe.TopProcs, value)
	}
	for key, value := range values {
		if service, ok := strings.CutPrefix(key, "service."); ok {
			probe.Services[service] = value
		}
	}
	return probe
}

func checkApps(ctx context.Context, alias string, apps []App) []AppCheck {
	results := []AppCheck{}
	var mutex sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, poolSize)
	for _, app := range apps {
		if strings.TrimSpace(app.URL) == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			status := checkApp(ctx, alias, app)
			mutex.Lock()
			results = append(results, status)
			mutex.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func checkApp(ctx context.Context, alias string, app App) AppCheck {
	status := AppCheck{Alias: alias, Name: app.Name, URL: app.URL, State: "down"}
	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, app.URL, nil)
	if err != nil {
		return status
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	start := time.Now()
	response, err := client.Do(request)
	status.Ms = time.Since(start).Milliseconds()
	if err != nil {
		return status
	}
	response.Body.Close()
	status.Code = response.StatusCode
	if response.StatusCode < 400 {
		status.State = "up"
	} else {
		status.State = "warn"
	}
	return status
}

func pveOf(alias string) (PVE, error) {
	if !ValidAlias(alias) {
		return PVE{}, errors.New("bad alias")
	}
	entry, ok := readSidecar()[alias]
	if !ok || entry.Pve == nil || entry.Pve.URL == "" || entry.Pve.Token == "" {
		return PVE{}, fmt.Errorf("no proxmox endpoint configured for %q", alias)
	}
	return *entry.Pve, nil
}

func pveDo(ctx context.Context, pve PVE, method, path string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(pve.URL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "PVEAPIToken="+pve.Token)
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: pve.Insecure}}}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New(strings.TrimSpace(response.Status))
	}
	return body, nil
}

func readSidecar() map[string]meta {
	result := map[string]meta{}
	data, err := os.ReadFile(xdg.RemotesFile())
	if err != nil {
		return result
	}
	_ = json.Unmarshal(data, &result)
	if result == nil {
		return map[string]meta{}
	}
	return result
}

func writeSidecar(side map[string]meta) error {
	data, err := json.MarshalIndent(side, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(xdg.RemotesFile(), append(data, '\n'), 0o600)
}

type includeBlock struct {
	alias string
	lines []string
}

func loadInclude(path string) ([]string, []includeBlock) {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"# Managed by ryoport. Do not edit; use the ryoport app or `ryossh add`/`ryossh remove`.", ""}, nil
	}
	header := []string{}
	blocks := []includeBlock{}
	current := -1
	for _, line := range strings.Split(string(data), "\n") {
		keyword, args := splitKV(line)
		if strings.EqualFold(keyword, "host") {
			blocks = append(blocks, includeBlock{alias: firstField(args), lines: []string{line}})
			current = len(blocks) - 1
		} else if current >= 0 {
			blocks[current].lines = append(blocks[current].lines, line)
		} else {
			header = append(header, line)
		}
	}
	return header, blocks
}

func writeInclude(path string, header []string, blocks []includeBlock) error {
	var content strings.Builder
	for _, line := range header {
		content.WriteString(strings.TrimRight(line, " \t"))
		content.WriteByte('\n')
	}
	for _, block := range blocks {
		for _, line := range block.lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			content.WriteString(strings.TrimRight(line, " \t"))
			content.WriteByte('\n')
		}
		content.WriteByte('\n')
	}
	return atomicWrite(path, []byte(content.String()), 0o600)
}

func ensureIncludeLine() error {
	data, err := os.ReadFile(xdg.SSHConfig())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return atomicWrite(xdg.SSHConfig(), []byte("Include config.d/*\n"), 0o600)
	}
	for _, line := range strings.Split(string(data), "\n") {
		keyword, args := splitKV(line)
		if strings.EqualFold(keyword, "include") {
			for _, field := range strings.Fields(args) {
				if field == "config.d/*" {
					return nil
				}
			}
		}
	}
	return atomicWrite(xdg.SSHConfig(), append([]byte("Include config.d/*\n"), data...), 0o600)
}

func stanzaLines(host HostInput) []string {
	lines := []string{"Host " + host.Alias, "    HostName " + host.HostName}
	if host.User != "" {
		lines = append(lines, "    User "+host.User)
	}
	port := host.Port
	if port == 0 {
		port = 22
	}
	lines = append(lines, "    Port "+strconv.Itoa(port))
	if host.IdentityFile != "" {
		lines = append(lines, "    IdentityFile "+host.IdentityFile)
	}
	if host.ProxyJump != "" {
		lines = append(lines, "    ProxyJump "+host.ProxyJump)
	}
	return lines
}

func readTunnels() []Tunnel {
	data, err := os.ReadFile(xdg.TunnelsFile())
	if err != nil {
		return nil
	}
	var tunnels []Tunnel
	if json.Unmarshal(data, &tunnels) != nil {
		return nil
	}
	return tunnels
}

func writeTunnels(tunnels []Tunnel) error {
	if tunnels == nil {
		tunnels = []Tunnel{}
	}
	data, err := json.MarshalIndent(tunnels, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(xdg.TunnelsFile(), append(data, '\n'), 0o600)
}

func ensureAskpass() (string, error) {
	path := xdg.AskpassFile()
	script := "#!/bin/sh\nexec secret-tool lookup " + secretKey + " \"$RYOPORT_ALIAS\"\n"
	if data, err := os.ReadFile(path); err == nil && string(data) == script {
		return path, nil
	}
	if err := atomicWrite(path, []byte(script), 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := xdg.EnsureDir(dir); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		backup := filepath.Join(dir, "."+filepath.Base(path)+".bak")
		_ = os.WriteFile(backup, old, mode)
	}
	temp, err := os.CreateTemp(dir, ".ryoport-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(name)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	remove = false
	return nil
}

func splitKV(line string) (string, string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] == '#' {
		return "", ""
	}
	index := strings.IndexAny(trimmed, " \t=")
	if index < 0 {
		return trimmed, ""
	}
	return trimmed[:index], strings.TrimLeft(trimmed[index:], " \t=")
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func validHost(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case char == '.' || char == '-' || char == '_':
		default:
			return false
		}
	}
	return true
}

func validUnit(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case char == '.' || char == '-' || char == '_' || char == '@' || char == ':':
		default:
			return false
		}
	}
	return true
}

func validWord(value string) bool {
	return validHost(value) && !strings.HasPrefix(value, "-")
}

func validKeyName(value string) bool {
	return validWord(value) && value != "." && value != ".."
}

func parseKV(value string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(value, "\n") {
		if key, item, ok := strings.Cut(line, "="); ok {
			result[key] = item
		}
	}
	return result
}

func parseKeyLine(line string) (bits int, fingerprint, kind, comment string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.Contains(fields[1], ":") {
		return 0, "", "", "", false
	}
	bits, _ = strconv.Atoi(fields[0])
	fingerprint = fields[1]
	last := fields[len(fields)-1]
	if strings.HasPrefix(last, "(") && strings.HasSuffix(last, ")") {
		kind = last[1 : len(last)-1]
		comment = strings.Join(fields[2:len(fields)-1], " ")
	} else {
		comment = strings.Join(fields[2:], " ")
	}
	return bits, fingerprint, kind, comment, true
}

func expandHome(path string) string {
	if path == "~" {
		return xdg.Home()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(xdg.Home(), path[2:])
	}
	return path
}

func shortenHome(path string) string {
	home := xdg.Home()
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~/" + path[len(home)+1:]
	}
	return path
}

func parallel(ctx context.Context, count int, fn func(int)) {
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				fn(i)
			case <-ctx.Done():
			}
		}()
	}
	wg.Wait()
}

func withDefaultTimeout(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, duration)
}

func parseInt(value string) int {
	result, _ := strconv.Atoi(strings.TrimSpace(value))
	return result
}

func parseInt64(value string) int64 {
	result, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return result
}

func parseFloat(value string) float64 {
	result, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return result
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func alive(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) == nil }

func pruneTunnels(tunnels []Tunnel) []Tunnel {
	live := make([]Tunnel, 0, len(tunnels))
	for _, tunnel := range tunnels {
		if alive(tunnel.Pid) {
			live = append(live, tunnel)
		}
	}
	return live
}

func killGroup(pid int) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func randomID() (string, error) {
	var data [4]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
