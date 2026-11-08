// Package qmp controls QEMU through its monitor and guest agent sockets.
package qmp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const (
	dialTimeout = 800 * time.Millisecond
	cmdDeadline = 1500 * time.Millisecond
	qgaDeadline = 600 * time.Millisecond
)

var (
	qemuPrompt = []byte("(qemu)")
	ansi       = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	reStatus   = regexp.MustCompile(`VM status:\s*(\S+)`)
	reBalloon  = regexp.MustCompile(`actual=(\d+)`)
	reThread   = regexp.MustCompile(`thread_id=(\d+)`)
)

// Stats is the live machine data emitted by the old ryovm-mon helper.
type Stats struct {
	Running        bool     `json:"running"`
	Status         string   `json:"status,omitempty"`
	BalloonMB      *int     `json:"balloonMB,omitempty"`
	VCPUs          *int     `json:"vcpus,omitempty"`
	VCPUTIDs       *[]int   `json:"vcpuTids,omitempty"`
	HostCPUPct     *float64 `json:"hostCpuPct,omitempty"`
	HostRSSMB      *int     `json:"hostRssMB,omitempty"`
	Pinned         *bool    `json:"pinned,omitempty"`
	DiskReadBytes  *uint64  `json:"diskReadBytes,omitempty"`
	DiskWriteBytes *uint64  `json:"diskWriteBytes,omitempty"`
	NetRXBytes     *uint64  `json:"netRxBytes,omitempty"`
	NetTXBytes     *uint64  `json:"netTxBytes,omitempty"`
	UptimeS        *int64   `json:"uptimeS,omitempty"`
	GuestIP        string   `json:"guestIp,omitempty"`
	GuestOS        string   `json:"guestOs,omitempty"`
}

// PowerAction is a monitor power command.
type PowerAction string

const (
	PowerPause    PowerAction = "pause"
	PowerResume   PowerAction = "resume"
	PowerReset    PowerAction = "reset"
	PowerShutdown PowerAction = "shutdown"
)

// PinAssignment records the host core selected for one vCPU thread.
type PinAssignment struct {
	TID  int `json:"tid"`
	Core int `json:"core"`
}

// PinResult is the JSON-compatible result of changing vCPU affinity.
type PinResult struct {
	OK     bool            `json:"ok"`
	Pinned []PinAssignment `json:"pinned"`
}

type monitor struct {
	conn net.Conn
}

func dialMonitor(ctx context.Context, sock string) (*monitor, error) {
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return nil, err
	}
	m := &monitor{conn: conn}
	_, _ = m.readReply(ctx)
	return m, nil
}

func (m *monitor) close() error { return m.conn.Close() }

func deadline(ctx context.Context, after time.Duration) time.Time {
	d := time.Now().Add(after)
	if cd, ok := ctx.Deadline(); ok && cd.Before(d) {
		return cd
	}
	return d
}

func (m *monitor) readReply(ctx context.Context) (string, error) {
	if err := m.conn.SetReadDeadline(deadline(ctx, cmdDeadline)); err != nil {
		return "", err
	}
	var buf [4096]byte
	acc := make([]byte, 0, 4096)
	for {
		n, err := m.conn.Read(buf[:])
		if n > 0 {
			acc = append(acc, buf[:n]...)
			if bytes.Contains(acc, qemuPrompt) {
				return ansi.ReplaceAllString(string(acc), ""), nil
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && len(acc) > 0 {
				return ansi.ReplaceAllString(string(acc), ""), nil
			}
			return ansi.ReplaceAllString(string(acc), ""), err
		}
	}
}

func (m *monitor) cmd(ctx context.Context, command string) (string, error) {
	if err := m.conn.SetWriteDeadline(deadline(ctx, cmdDeadline)); err != nil {
		return "", err
	}
	if _, err := m.conn.Write([]byte(command + "\n")); err != nil {
		return "", err
	}
	return m.readReply(ctx)
}

func parseStatus(reply string) string {
	if match := reStatus.FindStringSubmatch(reply); match != nil {
		return match[1]
	}
	return "unknown"
}

func parseBalloon(reply string) (int, bool) {
	if match := reBalloon.FindStringSubmatch(reply); match != nil {
		n, _ := strconv.Atoi(match[1])
		return n, true
	}
	return 0, false
}

func parseCPUs(reply string) []int {
	matches := reThread.FindAllStringSubmatch(reply, -1)
	tids := make([]int, 0, len(matches))
	for _, match := range matches {
		tid, _ := strconv.Atoi(match[1])
		tids = append(tids, tid)
	}
	return tids
}

// Collect reads monitor, host process and guest-agent statistics.
func Collect(ctx context.Context, monitorSock, agentSock, pidFile string) (Stats, error) {
	m, err := dialMonitor(ctx, monitorSock)
	if err != nil {
		return Stats{Running: false}, nil
	}
	defer m.close()

	stats := Stats{Running: true}
	reply, err := m.cmd(ctx, "info status")
	if err != nil {
		return Stats{}, fmt.Errorf("monitor: %w", err)
	}
	stats.Status = parseStatus(reply)
	if reply, err = m.cmd(ctx, "info balloon"); err != nil {
		return Stats{}, fmt.Errorf("monitor: %w", err)
	}
	if mb, ok := parseBalloon(reply); ok {
		stats.BalloonMB = &mb
	}
	if reply, err = m.cmd(ctx, "info cpus"); err != nil {
		return Stats{}, fmt.Errorf("monitor: %w", err)
	}
	tids := parseCPUs(reply)
	vcpus := len(tids)
	stats.VCPUs = &vcpus
	stats.VCPUTIDs = &tids

	if pidFile != "" && pidFile != "-" {
		if pid, readErr := readPID(pidFile); readErr == nil {
			if pct, ok := hostCPUPct(ctx, pid); ok {
				stats.HostCPUPct = &pct
			}
			if rss, ok := hostRSSMB(pid); ok {
				stats.HostRSSMB = &rss
			}
			pinned := allPinned(pid, tids)
			stats.Pinned = &pinned
			if io, ok := processIO(pid); ok {
				stats.DiskReadBytes = &io.diskRead
				stats.DiskWriteBytes = &io.diskWrite
				stats.NetRXBytes = &io.netRX
				stats.NetTXBytes = &io.netTX
			}
			if uptime, ok := processUptime(pid); ok {
				stats.UptimeS = &uptime
			}
		}
	}
	if agentSock != "" && agentSock != "-" {
		stats.GuestIP, stats.GuestOS = qgaInfo(ctx, agentSock)
	}
	return stats, nil
}

// Power sends a power action to a running machine.
func Power(ctx context.Context, monitorSock string, action PowerAction) error {
	var command string
	switch action {
	case PowerPause:
		command = "stop"
	case PowerResume:
		command = "cont"
	case PowerReset:
		command = "system_reset"
	case PowerShutdown:
		command = "system_powerdown"
	default:
		return errors.New("power action must be pause|resume|reset|shutdown")
	}
	m, err := dialMonitor(ctx, monitorSock)
	if err != nil {
		return fmt.Errorf("monitor: %w", err)
	}
	defer m.close()
	_, err = m.cmd(ctx, command)
	return err
}

// Balloon changes the guest-visible memory size in MiB.
func Balloon(ctx context.Context, monitorSock string, mb int) (int, error) {
	if mb <= 0 {
		return 0, errors.New("balloon MB must be a positive integer")
	}
	m, err := dialMonitor(ctx, monitorSock)
	if err != nil {
		return 0, fmt.Errorf("monitor: %w", err)
	}
	defer m.close()
	if _, err := m.cmd(ctx, "balloon "+strconv.Itoa(mb)); err != nil {
		return 0, err
	}
	return mb, nil
}

// Pin pins each vCPU to one host CPU, or restores access to every host CPU.
func Pin(ctx context.Context, monitorSock string, mode string) (PinResult, error) {
	if mode != "auto" && mode != "off" {
		return PinResult{}, errors.New("pin mode must be auto|off")
	}
	m, err := dialMonitor(ctx, monitorSock)
	if err != nil {
		return PinResult{}, fmt.Errorf("monitor: %w", err)
	}
	reply, cmdErr := m.cmd(ctx, "info cpus")
	_ = m.close()
	if cmdErr != nil {
		return PinResult{}, cmdErr
	}
	tids := parseCPUs(reply)
	if len(tids) == 0 {
		return PinResult{}, errors.New("no vcpus reported by monitor")
	}

	nproc := runtime.NumCPU()
	result := PinResult{OK: true, Pinned: make([]PinAssignment, 0, len(tids))}
	for i, tid := range tids {
		core := -1
		var set unix.CPUSet
		if mode == "auto" {
			core = i % nproc
			set.Set(core)
		} else {
			for cpu := range nproc {
				set.Set(cpu)
			}
		}
		if err := unix.SchedSetaffinity(tid, &set); err != nil {
			return PinResult{}, fmt.Errorf("setaffinity tid %d: %w", tid, err)
		}
		result.Pinned = append(result.Pinned, PinAssignment{TID: tid, Core: core})
	}
	return result, nil
}

func qgaInfo(ctx context.Context, sock string) (ip, osName string) {
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return "", ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline(ctx, qgaDeadline))
	dec := json.NewDecoder(conn)

	id := int(time.Now().UnixNano() & 0x7fffffff)
	if _, err = fmt.Fprintf(conn, "{\"execute\":\"guest-sync\",\"arguments\":{\"id\":%d}}\n", id); err != nil {
		return "", ""
	}
	for {
		var response struct {
			Return json.Number `json:"return"`
		}
		if dec.Decode(&response) != nil {
			return "", ""
		}
		n, _ := response.Return.Int64()
		if int(n) == id {
			break
		}
	}

	if _, err = fmt.Fprintln(conn, `{"execute":"guest-network-get-interfaces"}`); err != nil {
		return "", ""
	}
	var interfaces struct {
		Return []struct {
			Name string `json:"name"`
			IPs  []struct {
				Type string `json:"ip-address-type"`
				Addr string `json:"ip-address"`
			} `json:"ip-addresses"`
		} `json:"return"`
	}
	if dec.Decode(&interfaces) == nil {
		for _, iface := range interfaces.Return {
			if iface.Name == "lo" {
				continue
			}
			for _, address := range iface.IPs {
				if address.Type == "ipv4" && address.Addr != "127.0.0.1" {
					ip = address.Addr
					break
				}
			}
			if ip != "" {
				break
			}
		}
	}

	if _, err = fmt.Fprintln(conn, `{"execute":"guest-get-osinfo"}`); err != nil {
		return ip, ""
	}
	var osInfo struct {
		Return struct {
			PrettyName string `json:"pretty-name"`
		} `json:"return"`
	}
	if dec.Decode(&osInfo) == nil {
		osName = osInfo.Return.PrettyName
	}
	return ip, osName
}
