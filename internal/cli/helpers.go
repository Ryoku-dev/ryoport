package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ryoku-dev/ryoport/internal/fetch"
	"github.com/ryoku-dev/ryoport/internal/qmp"
)

func mainMonitor(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ryovm-mon: usage: ryovm-mon <stats|power|balloon|pin> ...")
		return 1
	}
	ctx := context.Background()
	switch args[0] {
	case "stats":
		if len(args) != 4 {
			return monitorUsage(stderr, "usage: ryovm-mon stats <monitorSock> <agentSock|-> <pidFile|->")
		}
		value, err := qmp.Collect(ctx, args[1], args[2], args[3])
		return encodeResult(stdout, stderr, monitorStats(value), err, "ryovm-mon")
	case "power":
		if len(args) != 3 {
			return monitorUsage(stderr, "usage: ryovm-mon power <monitorSock> <pause|resume|reset|shutdown>")
		}
		err := qmp.Power(ctx, args[1], qmp.PowerAction(args[2]))
		return encodeResult(stdout, stderr, struct {
			OK bool `json:"ok"`
		}{true}, err, "ryovm-mon")
	case "balloon":
		if len(args) != 3 {
			return monitorUsage(stderr, "usage: ryovm-mon balloon <monitorSock> <MB>")
		}
		mb, err := strconv.Atoi(args[2])
		if err != nil || mb <= 0 {
			return fail(stderr, "ryovm-mon", errors.New("balloon MB must be a positive integer"))
		}
		value, err := qmp.Balloon(ctx, args[1], mb)
		return encodeResult(stdout, stderr, struct {
			BalloonMB int  `json:"balloonMB"`
			OK        bool `json:"ok"`
		}{value, true}, err, "ryovm-mon")
	case "pin":
		if len(args) != 4 {
			return monitorUsage(stderr, "usage: ryovm-mon pin <monitorSock> <pidFile> <auto|off>")
		}
		data, err := os.ReadFile(args[2])
		if err != nil {
			return fail(stderr, "ryovm-mon", fmt.Errorf("pid: %w", err))
		}
		if _, err := strconv.Atoi(strings.TrimSpace(string(data))); err != nil {
			return fail(stderr, "ryovm-mon", fmt.Errorf("pid: %w", err))
		}
		value, err := qmp.Pin(ctx, args[1], args[3])
		return encodeResult(stdout, stderr, value, err, "ryovm-mon")
	default:
		return fail(stderr, "ryovm-mon", errors.New("unknown verb "+args[0]))
	}
}

func monitorStats(stats qmp.Stats) map[string]any {
	value := map[string]any{"running": stats.Running}
	if stats.Status != "" {
		value["status"] = stats.Status
	}
	if stats.BalloonMB != nil {
		value["balloonMB"] = *stats.BalloonMB
	}
	if stats.VCPUs != nil {
		value["vcpus"] = *stats.VCPUs
	}
	if stats.VCPUTIDs != nil {
		value["vcpuTids"] = *stats.VCPUTIDs
	}
	if stats.HostCPUPct != nil {
		value["hostCpuPct"] = *stats.HostCPUPct
	}
	if stats.HostRSSMB != nil {
		value["hostRssMB"] = *stats.HostRSSMB
	}
	if stats.Pinned != nil {
		value["pinned"] = *stats.Pinned
	}
	if stats.DiskReadBytes != nil {
		value["diskReadBytes"] = *stats.DiskReadBytes
	}
	if stats.DiskWriteBytes != nil {
		value["diskWriteBytes"] = *stats.DiskWriteBytes
	}
	if stats.NetRXBytes != nil {
		value["netRxBytes"] = *stats.NetRXBytes
	}
	if stats.NetTXBytes != nil {
		value["netTxBytes"] = *stats.NetTXBytes
	}
	if stats.UptimeS != nil {
		value["uptimeS"] = *stats.UptimeS
	}
	if stats.GuestIP != "" {
		value["guestIp"] = stats.GuestIP
	}
	if stats.GuestOS != "" {
		value["guestOs"] = stats.GuestOS
	}
	return value
}

func monitorUsage(stderr io.Writer, usage string) int {
	fmt.Fprintf(stderr, "ryovm-mon: %s\n", usage)
	return 1
}

func mainFetch(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_ = writeJSON(stdout, fetch.Event{Event: "error", Message: "usage: ryovm-fetch <url> <dest>"})
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := fetch.Stream(ctx, args[0], args[1], stdout); err != nil {
		return 1
	}
	return 0
}
