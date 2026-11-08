// Package cli implements the compatibility command names shipped by Ryoport.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/internal/version"
)

const ryoportUsage = "usage: ryoport [--select <id>] [--page fleet|activity|settings|create] | version | list|create|start|stop|remove|status [args] | <machine>"

// Main dispatches one compatibility tool. gui is true only when the Ryoport
// window should be opened.
func Main(argv []string, stdout, stderr io.Writer) (code int, gui bool) {
	if len(argv) == 0 {
		fmt.Fprintln(stderr, ryoportUsage)
		return 2, false
	}
	args := argv[1:]
	switch filepath.Base(argv[0]) {
	case "ryoport":
		return mainRyoport(args, stdout, stderr)
	case "ryovm":
		return mainRYOVM(args, stdout, stderr), false
	case "ryossh":
		return mainRYOSSH(args, stdout, stderr), false
	case "ryovm-mon":
		return mainMonitor(args, stdout, stderr), false
	case "ryovm-fetch":
		return mainFetch(args, stdout, stderr), false
	default:
		fmt.Fprintf(stderr, "ryoport: unknown command name %q\n", filepath.Base(argv[0]))
		return 2, false
	}
}

func mainRyoport(args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 {
		return 0, true
	}
	if args[0] == "--wait-pid" {
		if len(args) != 2 {
			fmt.Fprintln(stderr, ryoportUsage)
			return 2, false
		}
		pid, err := strconv.Atoi(args[1])
		if err != nil || pid <= 0 {
			fmt.Fprintln(stderr, "ryoport: --wait-pid needs a positive process id")
			return 2, false
		}
		waitPID(pid)
		return 0, true
	}
	if args[0] == "version" && len(args) == 1 {
		fmt.Fprintln(stdout, version.Version)
		return 0, false
	}
	if _, _, ok := parseWindow(args); ok {
		return 0, true
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, ryoportUsage)
		return 2, false
	}
	ctx := context.Background()
	switch args[0] {
	case "list", "create", "start", "stop", "remove", "status":
		out, err := passthrough.RunHub(ctx, args...)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1, false
		}
		_, _ = stdout.Write(out)
		return 0, false
	default:
		if len(args) != 1 {
			fmt.Fprintln(stderr, ryoportUsage)
			return 2, false
		}
		out, err := passthrough.RunHub(ctx, "start", "--name", args[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1, false
		}
		_, _ = stdout.Write(out)
		return 0, false
	}
}

// ParseWindow extracts the initial window route. Invalid input yields zero
// values; Main rejects it before asking the window to open.
func ParseWindow(argv []string) (selectID model.ID, page store.Page) {
	if len(argv) == 2 && argv[0] == "--wait-pid" {
		return "", store.PageFleet
	}
	selectID, page, _ = parseWindow(argv)
	return selectID, page
}

func parseWindow(argv []string) (model.ID, store.Page, bool) {
	page := store.PageFleet
	var selectID model.ID
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--select":
			if i+1 >= len(argv) || argv[i+1] == "" || strings.HasPrefix(argv[i+1], "-") {
				return "", "", false
			}
			i++
			selectID = model.ID(argv[i])
		case "--page":
			if i+1 >= len(argv) {
				return "", "", false
			}
			i++
			page = store.Page(argv[i])
			switch page {
			case store.PageFleet, store.PageActivity, store.PageSettings, store.PageCreate:
			default:
				return "", "", false
			}
		default:
			return "", "", false
		}
	}
	return selectID, page, true
}

func waitPID(pid int) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err != nil && err != syscall.EPERM {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func fail(stderr io.Writer, prefix string, err error) int {
	if prefix == "" {
		fmt.Fprintln(stderr, err)
	} else {
		fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
	}
	return 1
}

func need(args []string, count int, usage string, stderr io.Writer, prefix string) bool {
	if len(args) >= count {
		return true
	}
	if prefix != "" {
		fmt.Fprintf(stderr, "%s: %s\n", prefix, usage)
	} else {
		fmt.Fprintln(stderr, usage)
	}
	return false
}

func readPassword() string {
	line, _ := bufio.NewReader(io.LimitReader(os.Stdin, 1<<20)).ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
