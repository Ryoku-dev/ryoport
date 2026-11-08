package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

// CloudImage is one curated preinstalled cloud image.
type CloudImage struct {
	OS      string `json:"os"`
	Name    string `json:"name"`
	User    string `json:"user"`
	Size    string `json:"size"`
	Sudo    bool   `json:"sudo"`
	URL     string `json:"url,omitempty"`
	Resolve string `json:"resolve,omitempty"`
}

func Cloud() []CloudImage {
	return []CloudImage{
		{OS: "ubuntu", Name: "Ubuntu 24.04 LTS", User: "ubuntu", Size: "593M", URL: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"},
		{OS: "debian", Name: "Debian 13", User: "debian", Size: "325M", URL: "https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"},
		{OS: "fedora", Name: "Fedora Cloud", User: "fedora", Size: "557M", Resolve: "fedora"},
		{OS: "archlinux", Name: "Arch Linux", User: "arch", Size: "559M", URL: "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2"},
		{OS: "alpine", Name: "Alpine Linux", User: "alpine", Size: "175M", Sudo: true, URL: "https://dl-cdn.alpinelinux.org/alpine/v3.24/releases/cloud/generic_alpine-3.24.1-x86_64-uefi-cloudinit-r0.qcow2"},
		{OS: "opensuse", Name: "openSUSE Tumbleweed", User: "opensuse", Size: "293M", URL: "https://download.opensuse.org/tumbleweed/appliances/openSUSE-Tumbleweed-Minimal-VM.x86_64-Cloud.qcow2"},
		{OS: "rocky", Name: "Rocky Linux 10", User: "rocky", Size: "545M", URL: "https://dl.rockylinux.org/pub/rocky/10/images/x86_64/Rocky-10-GenericCloud-Base.latest.x86_64.qcow2"},
		{OS: "alma", Name: "AlmaLinux 10", User: "almalinux", Size: "573M", URL: "https://repo.almalinux.org/almalinux/10/cloud/x86_64/images/AlmaLinux-10-GenericCloud-latest.x86_64.qcow2"},
	}
}

// ResolveCloud resolves rolling catalogue entries such as Fedora releases.json.
func (b *Builder) ResolveCloud(ctx context.Context, image CloudImage) (string, error) {
	if image.Resolve == "" {
		if image.URL == "" {
			return "", errors.New("cloud image has no URL")
		}
		return image.URL, nil
	}
	if image.Resolve != "fedora" {
		return "", fmt.Errorf("unknown cloud image resolver %q", image.Resolve)
	}
	req, err := httpRequest(ctx, "https://fedoraproject.org/releases.json")
	if err != nil {
		return "", err
	}
	resp, err := b.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Fedora release catalogue replied %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}
	var rows []struct {
		Arch    string          `json:"arch"`
		Variant string          `json:"variant"`
		Link    string          `json:"link"`
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", err
	}
	type candidate struct{ version, link string }
	var choices []candidate
	for _, row := range rows {
		if row.Arch != "x86_64" || row.Variant != "Cloud" || !strings.HasSuffix(row.Link, ".qcow2") || !strings.Contains(row.Link, "Generic") || strings.Contains(row.Link, "UKI") {
			continue
		}
		choices = append(choices, candidate{version: strings.Trim(string(row.Version), `"`), link: row.Link})
	}
	if len(choices) == 0 {
		return "", errors.New("Fedora catalogue has no Generic Cloud qcow2")
	}
	sort.SliceStable(choices, func(i, j int) bool {
		return versionLess(choices[i].version, choices[j].version)
	})
	return choices[len(choices)-1].link, nil
}

func versionLess(left, right string) bool {
	leftNumber, leftErr := strconv.ParseFloat(left, 64)
	rightNumber, rightErr := strconv.ParseFloat(right, 64)
	if leftErr == nil && rightErr == nil {
		return leftNumber < rightNumber
	}
	return left < right
}

func httpRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err == nil {
		req.Header.Set("User-Agent", "ryovm-fetch/1")
	}
	return req, err
}

// ToolSet describes one cloud-init tool choice.
type ToolSet struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Heavy bool   `json:"heavy,omitempty"`
}

func ToolSets() []ToolSet {
	return []ToolSet{
		{ID: "git", Label: "git"},
		{ID: "build", Label: "build tools"},
		{ID: "python", Label: "python"},
		{ID: "node", Label: "node/npm"},
		{ID: "go", Label: "go"},
		{ID: "rust", Label: "rust"},
		{ID: "docker", Label: "docker", Heavy: true},
		{ID: "podman", Label: "podman", Heavy: true},
		{ID: "jq", Label: "jq"},
		{ID: "net", Label: "curl/wget"},
		{ID: "cli", Label: "htop·tmux·vim·rg"},
		{ID: "spice", Label: "SPICE clipboard"},
	}
}

var packageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+._-]*$`)

func family(osName string) string {
	switch osName {
	case "ubuntu", "debian":
		return "apt"
	case "fedora":
		return "dnf-fedora"
	case "rocky", "alma":
		return "dnf-rhel"
	case "archlinux":
		return "pacman"
	case "alpine":
		return "apk"
	case "opensuse":
		return "zypper"
	default:
		return "apt"
	}
}

func packages(tool, fam string) []string {
	switch tool {
	case "git":
		return []string{"git"}
	case "build":
		switch fam {
		case "apt":
			return []string{"build-essential"}
		case "pacman":
			return []string{"base-devel"}
		case "apk":
			return []string{"build-base"}
		default:
			return []string{"gcc", "gcc-c++", "make"}
		}
	case "python":
		switch fam {
		case "apt":
			return []string{"python3", "python3-pip", "python3-venv"}
		case "apk":
			return []string{"python3", "py3-pip"}
		case "pacman":
			return []string{"python", "python-pip"}
		default:
			return []string{"python3", "python3-pip"}
		}
	case "node":
		if fam == "zypper" {
			return []string{"nodejs-default", "npm-default"}
		}
		return []string{"nodejs", "npm"}
	case "go":
		if fam == "apt" {
			return []string{"golang-go"}
		}
		if strings.HasPrefix(fam, "dnf-") {
			return []string{"golang"}
		}
		return []string{"go"}
	case "rust":
		if fam == "apt" {
			return []string{"rustc", "cargo"}
		}
		if fam == "pacman" {
			return []string{"rust"}
		}
		return []string{"rust", "cargo"}
	case "docker":
		switch fam {
		case "apt":
			return []string{"docker.io", "docker-compose-v2"}
		case "dnf-fedora":
			return []string{"moby-engine", "docker-compose"}
		case "pacman":
			return []string{"docker", "docker-compose", "docker-buildx"}
		case "apk":
			return []string{"docker", "docker-cli-compose"}
		case "zypper":
			return []string{"docker", "docker-compose"}
		case "dnf-rhel":
			return nil
		}
	case "podman":
		return []string{"podman"}
	case "jq":
		return []string{"jq"}
	case "net":
		return []string{"curl", "wget"}
	case "cli":
		if fam == "apt" || strings.HasPrefix(fam, "dnf-") {
			return []string{"htop", "tmux", "vim", "ripgrep", "fd-find", "unzip"}
		}
		return []string{"htop", "tmux", "vim", "ripgrep", "fd", "unzip"}
	case "spice":
		return []string{"spice-vdagent", "qemu-guest-agent"}
	}
	return nil
}

func runCommands(tool, fam string) []string {
	switch tool {
	case "docker":
		switch fam {
		case "apk":
			return []string{"rc-update add docker default", "rc-service docker start", "addgroup ryoku docker"}
		case "dnf-rhel":
			return []string{"dnf -y install dnf-plugins-core", "dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo", "dnf -y install docker-ce docker-ce-cli containerd.io docker-compose-plugin", "systemctl enable --now docker", "groupadd -f docker", "usermod -aG docker ryoku"}
		default:
			return []string{"systemctl enable --now docker", "groupadd -f docker", "usermod -aG docker ryoku"}
		}
	case "spice":
		if fam == "apk" {
			return []string{"rc-update add spice-vdagentd default", "rc-service spice-vdagentd start"}
		}
		return []string{"systemctl enable --now spice-vdagentd"}
	}
	return nil
}

func ensureBurnKey(ctx context.Context) error {
	key := xdg.BurnKey()
	if _, err := os.Stat(key); err == nil {
		if _, pubErr := os.Stat(key + ".pub"); pubErr == nil {
			return nil
		}
		public, publicErr := exec.CommandContext(ctx, "ssh-keygen", "-y", "-f", key).Output()
		if publicErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("could not recover the burn public key")
		}
		return atomicWrite(key+".pub", append(public, '\n'), 0o644)
	}
	if err := xdg.EnsureDir(filepath.Dir(key)); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "ssh-keygen", "-t", "ed25519", "-N", "", "-C", "ryovm-burn", "-f", key, "-q").CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("could not create the burn key: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (b *Builder) writeSeed(ctx context.Context, dir, name string, wantSudo bool, osName string, tools, extras []string, base bool) error {
	if err := ensureBurnKey(ctx); err != nil {
		return err
	}
	validTools := make(map[string]bool)
	for _, tool := range ToolSets() {
		validTools[tool.ID] = true
	}
	fam := family(osName)
	var pkgs []string
	var commands []string
	if wantSudo {
		pkgs = append(pkgs, "sudo")
	}
	for _, tool := range tools {
		if !validTools[tool] {
			return fmt.Errorf("unknown tool set %q", tool)
		}
		pkgs = append(pkgs, packages(tool, fam)...)
		commands = append(commands, runCommands(tool, fam)...)
	}
	for _, pkg := range extras {
		pkg = strings.TrimSpace(pkg)
		if !packageName.MatchString(pkg) {
			return fmt.Errorf("invalid package name %q", pkg)
		}
		pkgs = append(pkgs, pkg)
	}
	if base {
		if fam == "apk" {
			pkgs = append(pkgs, "bash")
		}
		pkgs = append(pkgs, "bash-completion")
	}
	keyData, err := os.ReadFile(xdg.BurnKey() + ".pub")
	if err != nil {
		return err
	}
	keys := []string{strings.TrimSpace(string(keyData))}
	hostKeys, _ := filepath.Glob(filepath.Join(xdg.Home(), ".ssh", "id_*.pub"))
	for _, path := range hostKeys {
		if data, readErr := os.ReadFile(path); readErr == nil && strings.TrimSpace(string(data)) != "" {
			keys = append(keys, strings.TrimSpace(string(data)))
		}
	}
	var data strings.Builder
	fmt.Fprintf(&data, "#cloud-config\nhostname: %s\nssh_pwauth: true\n", name)
	fmt.Fprintf(&data, "package_update: %t\npackage_upgrade: false\n", len(pkgs) > 0 || len(commands) > 0)
	data.WriteString("chpasswd:\n  expire: false\nusers:\n  - name: ryoku\n    plain_text_passwd: ryoku\n    lock_passwd: false\n    sudo: \"ALL=(ALL) NOPASSWD:ALL\"\n    groups: [wheel, adm, sudo]\n    ssh_authorized_keys:\n")
	for _, key := range keys {
		fmt.Fprintf(&data, "      - %s\n", key)
	}
	data.WriteString("write_files:\n  - path: /usr/local/bin/clip\n    permissions: \"0755\"\n    content: |\n      #!/bin/sh\n      b64=$(base64 2>/dev/null | tr -d \"\\n\")\n      if [ -n \"$TMUX\" ]; then printf \"\\033Ptmux;\\033\\033]52;c;%s\\007\\033\\\\\" \"$b64\" > /dev/tty\n      else printf \"\\033]52;c;%s\\007\" \"$b64\" > /dev/tty; fi\n")
	if len(pkgs) > 0 {
		data.WriteString("packages:\n")
		for _, pkg := range pkgs {
			fmt.Fprintf(&data, "  - %s\n", pkg)
		}
	}
	if len(commands) > 0 {
		data.WriteString("runcmd:\n")
		for _, command := range commands {
			fmt.Fprintf(&data, "  - %s\n", command)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "user-data"), []byte(data.String()), 0o600); err != nil {
		return err
	}
	metadata := fmt.Sprintf("instance-id: iid-%s-%d\nlocal-hostname: %s\n", name, time.Now().Unix(), name)
	if err := os.WriteFile(filepath.Join(dir, "meta-data"), []byte(metadata), 0o600); err != nil {
		return err
	}
	return buildSeedISO(ctx, filepath.Join(dir, "seed.iso"), filepath.Join(dir, "user-data"), filepath.Join(dir, "meta-data"))
}

func buildSeedISO(ctx context.Context, output string, files ...string) error {
	for _, candidate := range []string{"genisoimage", "xorriso", "mkisofs"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		var args []string
		switch candidate {
		case "xorriso":
			args = []string{"-as", "mkisofs", "-quiet", "-o", output, "-V", "cidata", "-J", "-r"}
		default:
			args = []string{"-quiet", "-output", output, "-volid", "cidata", "-joliet", "-rock"}
		}
		args = append(args, files...)
		if result, err := exec.CommandContext(ctx, path, args...).CombinedOutput(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("build cloud-init seed: %s", strings.TrimSpace(string(result)))
		}
		return nil
	}
	return errors.New("no ISO builder found; install genisoimage, xorriso, or mkisofs")
}
