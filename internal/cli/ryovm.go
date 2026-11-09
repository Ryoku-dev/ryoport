package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

const ryovmUsage = "usage: ryovm caps|setup|catalog|cloud-catalog|list|get|create|instant|template|templates|spawn|import|virtio|launch|console|stop|config|rename|resize|snapshot|seal|restore-seal|usb|portfwd|ssh|delete|folder|icon|prefetch|paths|mon|lg"

func mainRYOVM(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, ryovmUsage)
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	manager := vm.New()
	builder := build.New(manager)
	verb, rest := args[0], args[1:]
	switch verb {
	case "caps":
		return encodeResult(stdout, stderr, manager.Caps(ctx), nil, "")
	case "setup":
		advice := vm.Setup()
		if len(advice.Argv) == 0 {
			return fail(stderr, "", errors.New(advice.Hint))
		}
		command := exec.CommandContext(ctx, advice.Argv[0], advice.Argv[1:]...)
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, stdout, stderr
		if err := command.Run(); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintln(stdout, "Done: quickemu is installed. Close this window and Ryoport picks it up when you return to it.")
		return 0
	case "catalog":
		refresh := len(rest) > 0 && rest[0] == "--refresh"
		data, err := builder.RawCatalog(ctx, refresh)
		if err != nil {
			return fail(stderr, "", err)
		}
		_, _ = stdout.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
		return 0
	case "cloud-catalog":
		return encodeResult(stdout, stderr, build.Cloud(), nil, "")
	case "list":
		value, err := manager.List(ctx)
		return encodeResult(stdout, stderr, value, err, "")
	case "get":
		if !need(rest, 1, "get needs a VM name", stderr, "") {
			return 1
		}
		value, err := manager.Get(ctx, rest[0])
		return encodeResult(stdout, stderr, value, err, "")
	case "create":
		if len(rest) < 2 {
			_ = writeJSON(stdout, build.Event{Phase: build.PhaseError, Message: "create needs <os> <release> [edition]"})
			return 1
		}
		edition := ""
		if len(rest) > 2 {
			edition = rest[2]
		}
		_, err := builder.Create(ctx, rest[0], rest[1], edition, buildEmitter(stdout))
		if err != nil {
			return 1
		}
		return 0
	case "instant":
		options, err := instantOptions(rest)
		if err != nil {
			_ = writeJSON(stdout, build.Event{Phase: build.PhaseError, Message: err.Error()})
			return 1
		}
		_, err = builder.Instant(ctx, options, buildEmitter(stdout))
		if err != nil {
			return 1
		}
		return 0
	case "template":
		if !need(rest, 1, "template needs a source VM", stderr, "") {
			return 1
		}
		name := rest[0]
		if len(rest) > 1 {
			name = rest[1]
		}
		if err := builder.MakeTemplate(ctx, rest[0], name); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "templated %s as '%s'\n", rest[0], name)
		return 0
	case "templates":
		templates, err := build.Templates()
		if err != nil {
			return fail(stderr, "", err)
		}
		type compatibleTemplate struct {
			OS    string `json:"os"`
			Tools string `json:"tools"`
			Base  string `json:"base"`
			Name  string `json:"name"`
		}
		value := make([]compatibleTemplate, 0, len(templates))
		for _, template := range templates {
			value = append(value, compatibleTemplate{template.OS, template.Tools, template.Base, template.Name})
		}
		return encodeResult(stdout, stderr, value, nil, "")
	case "spawn":
		if len(rest) == 0 {
			_ = writeJSON(stdout, build.Event{Phase: build.PhaseError, Message: "spawn needs a template"})
			return 1
		}
		tmpl, name, disposable := "", "", false
		for _, arg := range rest {
			if arg == "--disposable" {
				disposable = true
			} else if tmpl == "" {
				tmpl = arg
			} else {
				name = arg
			}
		}
		_ = writeJSON(stdout, build.Event{Phase: build.PhaseConfig})
		made, err := builder.Spawn(ctx, tmpl, name, disposable)
		if err != nil {
			_ = writeJSON(stdout, build.Event{Phase: build.PhaseError, Message: err.Error()})
			return 1
		}
		_ = writeJSON(stdout, build.Event{Phase: build.PhaseDone, Name: made})
		return 0
	case "import":
		if !need(rest, 2, "import needs a name and ISO path", stderr, "") {
			return 1
		}
		guest, osName := "linux", ""
		if len(rest) > 2 {
			guest = rest[2]
		}
		if len(rest) > 3 {
			osName = rest[3]
		}
		if err := builder.Import(rest[0], rest[1], guest, osName); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "Created %s\n", manager.ConfPath(rest[0]))
		return 0
	case "virtio":
		name := ""
		if len(rest) > 0 {
			name = rest[0]
		}
		baseEmit := buildEmitter(stdout)
		emit := func(event build.Event) {
			if event.Phase != build.PhaseDone {
				baseEmit(event)
			}
		}
		path, err := builder.VirtIO(ctx, name, emit)
		if err != nil {
			return 1
		}
		return encodeResult(stdout, stderr, struct {
			Phase build.Phase `json:"phase"`
			ISO   string      `json:"iso"`
		}{build.PhaseDone, path}, nil, "")
	case "launch", "start":
		if !need(rest, 1, "launch needs a VM name", stderr, "") {
			return 1
		}
		display := vm.DisplayWindow
		if len(rest) > 1 {
			display = vm.Display(rest[1])
		}
		disposable := len(rest) > 2 && rest[2] == "--disposable"
		if err := manager.Launch(ctx, rest[0], display, disposable); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "launched %s (%s)\n", rest[0], display)
		return 0
	case "console", "viewer":
		if !need(rest, 1, "console needs a VM name", stderr, "") {
			return 1
		}
		if err := manager.OpenViewer(rest[0]); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "console opened for %s\n", rest[0])
		return 0
	case "stop":
		if !need(rest, 1, "stop needs a VM name", stderr, "") {
			return 1
		}
		force := len(rest) > 1 && rest[1] == "--force"
		if err := manager.Stop(ctx, rest[0], force); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "stopped %s\n", rest[0])
		return 0
	case "config":
		if !need(rest, 2, "config needs <name> <key> [value]", stderr, "") {
			return 1
		}
		value := ""
		if len(rest) > 2 {
			value = rest[2]
		}
		if err := manager.SetConfig(rest[0], rest[1], value); err != nil {
			return fail(stderr, "", err)
		}
		if value == "" || value == "auto" {
			fmt.Fprintf(stdout, "cleared %s on %s\n", rest[1], rest[0])
		} else {
			fmt.Fprintf(stdout, "set %s=%s on %s\n", rest[1], value, rest[0])
		}
		return 0
	case "rename":
		if !need(rest, 2, "rename needs <old> <new>", stderr, "") {
			return 1
		}
		if err := manager.Rename(rest[0], rest[1]); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "renamed %s to %s\n", rest[0], rest[1])
		return 0
	case "resize":
		if !need(rest, 2, "resize needs a size (e.g. 80G)", stderr, "") {
			return 1
		}
		if err := manager.Resize(ctx, rest[0], rest[1]); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "resized %s disk to %s\n", rest[0], rest[1])
		return 0
	case "snapshot":
		return snapshotCommand(ctx, manager, rest, stdout, stderr)
	case "seal":
		if !need(rest, 1, "seal needs a VM name", stderr, "") {
			return 1
		}
		if err := manager.Seal(ctx, rest[0]); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "sealed %s at its current state\n", rest[0])
		return 0
	case "restore-seal":
		if !need(rest, 1, "restore-seal needs a VM name", stderr, "") {
			return 1
		}
		if err := manager.RestoreSeal(ctx, rest[0]); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "restored %s to its sealed state\n", rest[0])
		return 0
	case "usb":
		return usbCommand(ctx, manager, rest, stdout, stderr)
	case "portfwd":
		return portCommand(manager, rest, stdout, stderr)
	case "ssh":
		if !need(rest, 1, "ssh needs a VM name", stderr, "") {
			return 1
		}
		argv, err := manager.SSHCommand(rest[0])
		if err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintln(stdout, strings.Join(argv, " "))
		return 0
	case "delete":
		if !need(rest, 1, "delete needs a VM name", stderr, "") {
			return 1
		}
		diskOnly := len(rest) > 1 && rest[1] == "--disk-only"
		if err := manager.Delete(ctx, rest[0], diskOnly); err != nil {
			return fail(stderr, "", err)
		}
		if diskOnly {
			fmt.Fprintf(stdout, "reclaimed the disk for %s\n", rest[0])
		} else {
			fmt.Fprintf(stdout, "deleted %s\n", rest[0])
		}
		return 0
	case "folder":
		name := ""
		if len(rest) > 0 {
			name = rest[0]
			if _, err := manager.DiskPath(name); err != nil {
				return fail(stderr, "", err)
			}
		} else if err := os.MkdirAll(manager.Root, 0o700); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintln(stdout, manager.Folder(name))
		return 0
	case "icon":
		if !need(rest, 1, "icon needs an OS slug", stderr, "") {
			return 1
		}
		path, err := builder.Icon(ctx, rest[0])
		if err != nil {
			return fail(stderr, "", err)
		}
		if path != "" {
			fmt.Fprintln(stdout, path)
		}
		return 0
	case "prefetch":
		data, err := builder.RawCatalog(ctx, false)
		if err != nil {
			return fail(stderr, "", err)
		}
		var rows []struct {
			OS string `json:"OS"`
		}
		if err := json.Unmarshal(data, &rows); err != nil {
			return fail(stderr, "", err)
		}
		oses := make([]string, 0, len(rows))
		for _, row := range rows {
			oses = append(oses, row.OS)
		}
		return encodeResult(stdout, stderr, builder.Prefetch(ctx, oses), nil, "")
	case "paths":
		return encodeResult(stdout, stderr, manager.Paths(), nil, "")
	case "mon":
		return machineMonitor(ctx, manager, rest, stdout, stderr)
	case "lg":
		return lgCommand(ctx, rest, stdout, stderr)
	default:
		fmt.Fprintln(stderr, ryovmUsage)
		return 2
	}
}

func buildEmitter(stdout io.Writer) func(build.Event) {
	return func(event build.Event) {
		if event.Progress != nil {
			_ = writeJSON(stdout, fetch.Event{Event: "progress", Recv: event.Progress.Recv, Total: event.Progress.Total, BPS: event.Progress.BPS})
			return
		}
		_ = writeJSON(stdout, event)
	}
}

func instantOptions(args []string) (build.InstantOptions, error) {
	var options build.InstantOptions
	for _, arg := range args {
		switch {
		case arg == "--disposable":
			options.Disposable = true
		case arg == "--sealed":
			options.Sealed = true
		case strings.HasPrefix(arg, "--tools="):
			options.Tools = splitCSV(strings.TrimPrefix(arg, "--tools="))
		case strings.HasPrefix(arg, "--pkgs="):
			options.Pkgs = splitCSV(strings.TrimPrefix(arg, "--pkgs="))
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown instant flag %s", arg)
		case options.OS == "":
			options.OS = arg
		case options.Name == "":
			options.Name = arg
		default:
			return options, errors.New("instant accepts one OS and optional name")
		}
	}
	if options.OS == "" {
		return options, errors.New("instant needs an OS (e.g. debian)")
	}
	return options, nil
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func snapshotCommand(ctx context.Context, manager *vm.Manager, args []string, stdout, stderr io.Writer) int {
	if !need(args, 2, "snapshot needs <name> list|create|restore|delete", stderr, "") {
		return 1
	}
	name, action := args[0], args[1]
	if action == "list" {
		value, err := manager.Snapshots(ctx, name)
		return encodeResult(stdout, stderr, value, err, "")
	}
	if !need(args, 3, action+" needs a tag", stderr, "") {
		return 1
	}
	var err error
	switch action {
	case "create":
		err = manager.CreateSnapshot(ctx, name, args[2])
	case "restore":
		err = manager.RestoreSnapshot(ctx, name, args[2])
	case "delete":
		err = manager.DeleteSnapshot(ctx, name, args[2])
	default:
		return fail(stderr, "", fmt.Errorf("unknown snapshot subcommand: %s", action))
	}
	if err != nil {
		return fail(stderr, "", err)
	}
	words := map[string]string{"create": "snapshot '" + args[2] + "' created", "restore": "restored '" + args[2] + "'", "delete": "deleted '" + args[2] + "'"}
	fmt.Fprintln(stdout, words[action])
	return 0
}

func usbCommand(ctx context.Context, manager *vm.Manager, args []string, stdout, stderr io.Writer) int {
	if !need(args, 2, "usb needs list|set and a VM name", stderr, "") {
		return 1
	}
	switch args[0] {
	case "list":
		value, err := manager.USB(ctx, args[1])
		return encodeResult(stdout, stderr, value, err, "")
	case "set":
		if !need(args, 4, "usb set needs <name> <vid:pid> on|off", stderr, "") {
			return 1
		}
		if args[3] != "on" && args[3] != "off" {
			return fail(stderr, "", errors.New("usb set needs on|off"))
		}
		if err := manager.SetUSB(args[1], args[2], args[3] == "on"); err != nil {
			return fail(stderr, "", err)
		}
		fmt.Fprintf(stdout, "usb %s %s for %s\n", args[2], args[3], args[1])
		return 0
	default:
		return fail(stderr, "", fmt.Errorf("unknown usb subcommand: %s", args[0]))
	}
}

func portCommand(manager *vm.Manager, args []string, stdout, stderr io.Writer) int {
	if !need(args, 2, "portfwd needs list|add|remove and a VM name", stderr, "") {
		return 1
	}
	if args[0] == "list" {
		value, err := manager.PortForwards(args[1])
		return encodeResult(stdout, stderr, value, err, "")
	}
	if !need(args, 3, "portfwd "+args[0]+" needs <hostport:guestport>", stderr, "") {
		return 1
	}
	var err error
	switch args[0] {
	case "add":
		err = manager.AddPortForward(args[1], args[2])
	case "remove":
		err = manager.RemovePortForward(args[1], args[2])
	default:
		return fail(stderr, "", fmt.Errorf("unknown portfwd subcommand: %s", args[0]))
	}
	if err != nil {
		return fail(stderr, "", err)
	}
	fmt.Fprintf(stdout, "port forwards updated for %s\n", args[1])
	return 0
}

func machineMonitor(ctx context.Context, manager *vm.Manager, args []string, stdout, stderr io.Writer) int {
	if !need(args, 1, "mon needs a VM name", stderr, "") {
		return 1
	}
	action := "stats"
	if len(args) > 1 {
		action = args[1]
	}
	switch action {
	case "stats":
		value, err := manager.Stats(ctx, args[0])
		return encodeResult(stdout, stderr, monitorStats(value), err, "")
	case "power":
		if !need(args, 3, "power needs pause|resume|reset|shutdown", stderr, "") {
			return 1
		}
		err := manager.Power(ctx, args[0], qmp.PowerAction(args[2]))
		return encodeResult(stdout, stderr, struct {
			OK bool `json:"ok"`
		}{true}, err, "")
	case "balloon":
		if !need(args, 3, "balloon needs a size in MB", stderr, "") {
			return 1
		}
		mb, err := strconv.Atoi(args[2])
		if err != nil {
			return fail(stderr, "", errors.New("balloon MB must be a positive integer"))
		}
		err = manager.Balloon(ctx, args[0], mb)
		return encodeResult(stdout, stderr, struct {
			BalloonMB int  `json:"balloonMB"`
			OK        bool `json:"ok"`
		}{mb, true}, err, "")
	case "pin":
		mode := "auto"
		if len(args) > 2 {
			mode = args[2]
		}
		disk, err := manager.DiskPath(args[0])
		if err != nil {
			return fail(stderr, "", err)
		}
		value, err := qmp.Pin(ctx, filepath.Join(filepath.Dir(disk), args[0]+"-monitor.socket"), mode)
		return encodeResult(stdout, stderr, value, err, "")
	default:
		return fail(stderr, "", fmt.Errorf("mon: unknown subcommand '%s'", action))
	}
}

func lgCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	if sub == "stats" {
		if !need(args, 2, "lg stats needs a machine name", stderr, "") {
			return 1
		}
		value, err := passthrough.ReadStats(ctx, args[1])
		return encodeResult(stdout, stderr, value, err, "")
	}
	var hubArgs []string
	switch sub {
	case "list", "create", "start", "stop", "remove", "status":
		hubArgs = args
		if len(hubArgs) == 0 {
			hubArgs = []string{"list"}
		}
	default:
		hubArgs = []string{"start", "--name", sub}
	}
	out, err := passthrough.RunHub(ctx, hubArgs...)
	if err != nil {
		return fail(stderr, "", err)
	}
	_, _ = stdout.Write(out)
	return 0
}

func encodeResult(stdout, stderr io.Writer, value any, err error, prefix string) int {
	if err != nil {
		return fail(stderr, prefix, err)
	}
	if err := writeJSON(stdout, value); err != nil {
		return fail(stderr, prefix, err)
	}
	return 0
}
