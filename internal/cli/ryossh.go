package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/keyring"
	"github.com/ryoku-dev/ryoport/internal/remote"
)

const ryosshUsage = "usage: ryossh <list|ping|pingall|probe|probeall|appcheck|appcheckall|pveguests|pveaction|connect|keys|copyid|keygen|setpass|clearpass|add|remove|knownremove|tunnel|console> [args]"

func mainRYOSSH(args []string, stdout, stderr io.Writer) int {
	askpassEnv := os.Getenv("RYOPORT_ALIAS") != "" && os.Getenv("SSH_ASKPASS_REQUIRE") == "force"
	askpassArg := len(args) > 0 && args[0] == "askpass"
	askpassPrompt := askpassEnv && len(args) <= 1 && (len(args) == 0 || !ryosshVerb(args[0]))
	if askpassArg || askpassPrompt || os.Getenv("RYOPORT_ASKPASS") == "1" {
		return askpass(stdout, stderr)
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "ryossh: %s\n", ryosshUsage)
		return 1
	}
	ctx := context.Background()
	manager := remote.New()
	verb, rest := args[0], args[1:]
	switch verb {
	case "list":
		value, err := manager.List(ctx)
		return encodeResult(stdout, stderr, value, err, "ryossh")
	case "ping":
		if !sshNeed(rest, 1, "usage: ryossh ping <alias>", stderr) {
			return 1
		}
		return encodeResult(stdout, stderr, remoteReachValue(manager.Ping(ctx, rest[0])), nil, "ryossh")
	case "pingall":
		reaches := manager.PingAll(ctx)
		values := make([]map[string]any, 0, len(reaches))
		for _, reach := range reaches {
			values = append(values, remoteReachValue(reach))
		}
		return encodeResult(stdout, stderr, values, nil, "ryossh")
	case "probe":
		if !sshNeed(rest, 1, "usage: ryossh probe <alias>", stderr) {
			return 1
		}
		probe := manager.Probe(ctx, rest[0])
		if code := encodeResult(stdout, stderr, remoteProbeValue(probe), nil, "ryossh"); code != 0 {
			return code
		}
		if !probe.OK {
			return 1
		}
		return 0
	case "probeall":
		probes := manager.ProbeAll(ctx)
		values := make([]any, 0, len(probes))
		for _, probe := range probes {
			values = append(values, remoteProbeValue(probe))
		}
		return encodeResult(stdout, stderr, values, nil, "ryossh")
	case "appcheck":
		if !sshNeed(rest, 1, "usage: ryossh appcheck <alias>", stderr) {
			return 1
		}
		return encodeResult(stdout, stderr, manager.AppCheck(ctx, rest[0]), nil, "ryossh")
	case "appcheckall":
		return encodeResult(stdout, stderr, manager.AppCheckAll(ctx), nil, "ryossh")
	case "pveguests":
		if !sshNeed(rest, 1, "usage: ryossh pveguests <alias>", stderr) {
			return 1
		}
		value, err := manager.PVEGuests(ctx, rest[0])
		return encodeResult(stdout, stderr, value, err, "ryossh")
	case "pveaction":
		if !sshNeed(rest, 5, "usage: ryossh pveaction <alias> <node> <qemu|lxc> <vmid> <start|stop|shutdown|reboot>", stderr) {
			return 1
		}
		vmid, err := strconv.Atoi(rest[3])
		if err != nil {
			return fail(stderr, "ryossh", errors.New("vmid must be a number"))
		}
		if err := manager.PVEAction(ctx, rest[0], rest[1], rest[2], vmid, rest[4]); err != nil {
			return fail(stderr, "ryossh", fmt.Errorf("proxmox: %w", err))
		}
		return okJSON(stdout, stderr)
	case "connect":
		if !sshNeed(rest, 1, "usage: ryossh connect <alias> [--cmd]", stderr) {
			return 1
		}
		if len(rest) > 1 && rest[1] == "--cmd" {
			if !remote.ValidAlias(rest[0]) {
				return fail(stderr, "ryossh", errors.New("bad alias"))
			}
			return encodeResult(stdout, stderr, []string{"ssh", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", rest[0]}, nil, "ryossh")
		}
		if err := manager.Connect(rest[0]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return encodeResult(stdout, stderr, map[string]any{
			"alias": rest[0], "launched": true,
		}, nil, "ryossh")
	case "keys":
		value, err := manager.Keys(ctx)
		return encodeResult(stdout, stderr, value, err, "ryossh")
	case "copyid":
		if !sshNeed(rest, 1, "usage: ryossh copyid <alias> [pubkey]", stderr) {
			return 1
		}
		publicKey := ""
		if len(rest) > 1 {
			publicKey = rest[1]
		}
		if err := manager.CopyID(rest[0], publicKey); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	case "keygen":
		if !sshNeed(rest, 1, "usage: ryossh keygen <name> [type]", stderr) {
			return 1
		}
		kind := "ed25519"
		if len(rest) > 1 && rest[1] != "" {
			kind = rest[1]
		}
		path, err := manager.Keygen(rest[0], kind)
		if err != nil {
			return fail(stderr, "ryossh", err)
		}
		fingerprint := ""
		if keys, keyErr := manager.Keys(ctx); keyErr == nil {
			for _, key := range keys.Files {
				if key.Path == path {
					fingerprint = key.Fingerprint
					break
				}
			}
		}
		return encodeResult(stdout, stderr, map[string]any{
			"fingerprint": fingerprint, "ok": true, "path": path,
		}, nil, "ryossh")
	case "setpass":
		if !sshNeed(rest, 1, "usage: ryossh setpass <alias>", stderr) {
			return 1
		}
		password := readPassword()
		if password == "" {
			return fail(stderr, "ryossh", errors.New("no password on stdin"))
		}
		if err := manager.SetPassword(rest[0], password); err != nil {
			return fail(stderr, "ryossh", passwordStoreError(err))
		}
		return aliasOK(stdout, stderr, rest[0])
	case "clearpass":
		if !sshNeed(rest, 1, "usage: ryossh clearpass <alias>", stderr) {
			return 1
		}
		if err := manager.ClearPassword(rest[0]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return aliasOK(stdout, stderr, rest[0])
	case "add":
		if !sshNeed(rest, 1, "usage: ryossh add <json>", stderr) {
			return 1
		}
		var input remote.HostInput
		if err := json.Unmarshal([]byte(rest[0]), &input); err != nil {
			return fail(stderr, "ryossh", fmt.Errorf("invalid host JSON: %w", err))
		}
		if err := manager.Save(input); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	case "remove":
		if !sshNeed(rest, 1, "usage: ryossh remove <alias>", stderr) {
			return 1
		}
		if err := manager.Remove(rest[0]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	case "knownremove":
		if !sshNeed(rest, 1, "usage: ryossh knownremove <alias>", stderr) {
			return 1
		}
		if err := manager.KnownRemove(rest[0]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	case "tunnel":
		return tunnelCommand(manager, rest, stdout, stderr)
	case "console":
		return consoleCommand(ctx, console.New(), rest, stdout, stderr)
	default:
		return fail(stderr, "ryossh", fmt.Errorf("unknown verb %q", verb))
	}
}

func askpass(stdout, stderr io.Writer) int {
	alias := os.Getenv("RYOPORT_ALIAS")
	if alias == "" {
		return fail(stderr, "ryossh", errors.New("RYOPORT_ALIAS is not set"))
	}
	secret, err := keyring.Lookup(map[string]string{"ryoport-alias": alias})
	if err != nil {
		return fail(stderr, "ryossh", err)
	}
	fmt.Fprintln(stdout, secret)
	return 0
}

func sshNeed(args []string, count int, usage string, stderr io.Writer) bool {
	if len(args) >= count {
		return true
	}
	fmt.Fprintf(stderr, "ryossh: %s\n", usage)
	return false
}

func okJSON(stdout, stderr io.Writer) int {
	return encodeResult(stdout, stderr, struct {
		OK bool `json:"ok"`
	}{true}, nil, "ryossh")
}

func aliasOK(stdout, stderr io.Writer, alias string) int {
	return encodeResult(stdout, stderr, map[string]any{
		"alias": alias, "ok": true,
	}, nil, "ryossh")
}

func tunnelCommand(manager *remote.Manager, args []string, stdout, stderr io.Writer) int {
	if !sshNeed(args, 1, "usage: ryossh tunnel <open|list|close|closeall> [args]", stderr) {
		return 1
	}
	switch args[0] {
	case "open":
		if !sshNeed(args, 3, "usage: ryossh tunnel open <alias> <spec>", stderr) {
			return 1
		}
		tunnel, err := manager.OpenTunnel(args[1], args[2])
		if err != nil {
			return fail(stderr, "ryossh", fmt.Errorf("tunnel failed: %w", err))
		}
		return encodeResult(stdout, stderr, map[string]any{
			"id": tunnel.ID, "kind": tunnel.Kind, "ok": true, "pid": tunnel.Pid, "spec": tunnel.Spec,
		}, nil, "ryossh")
	case "list":
		alias := ""
		if len(args) > 1 {
			alias = args[1]
		}
		value, err := manager.Tunnels(alias)
		return encodeResult(stdout, stderr, value, err, "ryossh")
	case "close":
		if !sshNeed(args, 2, "usage: ryossh tunnel close <id>", stderr) {
			return 1
		}
		if err := manager.CloseTunnel(args[1]); err != nil {
			_ = writeJSON(stdout, map[string]any{"error": err.Error(), "ok": false})
			return 1
		}
		return okJSON(stdout, stderr)
	case "closeall":
		alias := ""
		if len(args) > 1 {
			alias = args[1]
		}
		tunnels, err := manager.Tunnels(alias)
		if err != nil {
			return fail(stderr, "ryossh", err)
		}
		if err := manager.CloseAll(alias); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return encodeResult(stdout, stderr, map[string]any{
			"closed": len(tunnels), "ok": true,
		}, nil, "ryossh")
	default:
		return fail(stderr, "ryossh", fmt.Errorf("unknown tunnel verb %q", args[0]))
	}
}

func consoleCommand(ctx context.Context, manager *console.Manager, args []string, stdout, stderr io.Writer) int {
	if !sshNeed(args, 1, "usage: ryossh console <list|add|remove|open|probe|probeall|caps|setpass|clearpass> [args]", stderr) {
		return 1
	}
	switch args[0] {
	case "list":
		value, err := manager.List()
		return encodeResult(stdout, stderr, value, err, "ryossh")
	case "caps":
		return encodeResult(stdout, stderr, manager.Caps(), nil, "ryossh")
	case "add":
		if !sshNeed(args, 2, "usage: ryossh console add <json>", stderr) {
			return 1
		}
		var profile console.Profile
		if err := json.Unmarshal([]byte(args[1]), &profile); err != nil {
			return fail(stderr, "ryossh", fmt.Errorf("invalid console JSON: %w", err))
		}
		if profile.ID == "" {
			profile.ID = console.NewID()
		}
		if err := manager.Save(profile); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return encodeResult(stdout, stderr, map[string]any{
			"id": profile.ID, "ok": true,
		}, nil, "ryossh")
	case "remove":
		if !sshNeed(args, 2, "usage: ryossh console remove <id>", stderr) {
			return 1
		}
		if err := manager.Remove(args[1]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	case "open":
		if !sshNeed(args, 2, "usage: ryossh console open <id>", stderr) {
			return 1
		}
		profiles, _ := manager.List()
		kind := ""
		for _, profile := range profiles {
			if profile.ID == args[1] {
				kind = profile.Kind
				break
			}
		}
		if err := manager.Open(args[1]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return encodeResult(stdout, stderr, map[string]any{
			"id": args[1], "kind": kind, "launched": true,
		}, nil, "ryossh")
	case "probe":
		if !sshNeed(args, 2, "usage: ryossh console probe <id>", stderr) {
			return 1
		}
		profiles, _ := manager.List()
		found := false
		for _, profile := range profiles {
			if profile.ID == args[1] {
				found = true
				break
			}
		}
		if !found {
			return fail(stderr, "ryossh", errors.New("no such console"))
		}
		reach := manager.Probe(ctx, args[1])
		if err := writeJSON(stdout, consoleReachValue(reach)); err != nil {
			return fail(stderr, "ryossh", err)
		}
		if !reach.Up {
			return 1
		}
		return 0
	case "probeall":
		reaches := manager.ProbeAll(ctx)
		values := make([]map[string]any, 0, len(reaches))
		for _, reach := range reaches {
			values = append(values, consoleReachValue(reach))
		}
		return encodeResult(stdout, stderr, values, nil, "ryossh")
	case "setpass":
		if !sshNeed(args, 2, "usage: ryossh console setpass <id>", stderr) {
			return 1
		}
		password := readPassword()
		if password == "" {
			return fail(stderr, "ryossh", errors.New("no password on stdin"))
		}
		if err := manager.SetPassword(args[1], password); err != nil {
			return fail(stderr, "ryossh", passwordStoreError(err))
		}
		return encodeResult(stdout, stderr, map[string]any{
			"id": args[1], "ok": true,
		}, nil, "ryossh")
	case "clearpass":
		if !sshNeed(args, 2, "usage: ryossh console clearpass <id>", stderr) {
			return 1
		}
		if err := manager.ClearPassword(args[1]); err != nil {
			return fail(stderr, "ryossh", err)
		}
		return okJSON(stdout, stderr)
	default:
		return fail(stderr, "ryossh", fmt.Errorf("unknown console verb %q", args[0]))
	}
}

func consoleReachValue(reach console.Reach) map[string]any {
	value := map[string]any{
		"id": reach.ID, "kind": reach.Kind, "rttMs": reach.RTTMs, "up": reach.Up,
	}
	if reach.Error != "" {
		value["error"] = reach.Error
	}
	return value
}

func remoteReachValue(reach remote.Reach) map[string]any {
	return map[string]any{
		"alias": reach.Alias, "rttMs": reach.RTTMs, "sshUp": reach.SSHUp, "up": reach.Up,
	}
}

func remoteProbeValue(probe remote.Probe) any {
	if probe.OK {
		return probe
	}
	return map[string]any{
		"alias": probe.Alias, "error": probe.Error, "needAuth": probe.NeedAuth, "ok": false,
	}
}

func passwordStoreError(err error) error {
	if strings.HasPrefix(err.Error(), "secret-tool not found:") {
		return err
	}
	return fmt.Errorf("keyring store failed: %w", err)
}

func ryosshVerb(value string) bool {
	switch value {
	case "list", "ping", "pingall", "probe", "probeall", "appcheck", "appcheckall",
		"pveguests", "pveaction", "connect", "keys", "copyid", "keygen", "setpass",
		"clearpass", "add", "remove", "knownremove", "tunnel", "console":
		return true
	}
	return false
}
