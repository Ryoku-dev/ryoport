package qmp

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const cpuSample = 300 * time.Millisecond

func readPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func procJiffies(pid int) (int64, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	text := string(data)
	end := strings.LastIndexByte(text, ')')
	if end < 0 {
		return 0, false
	}
	fields := strings.Fields(text[end+1:])
	if len(fields) < 13 {
		return 0, false
	}
	user, _ := strconv.ParseInt(fields[11], 10, 64)
	system, _ := strconv.ParseInt(fields[12], 10, 64)
	return user + system, true
}

func totalJiffies() (int64, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	line := string(data)
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "cpu" {
		return 0, false
	}
	var total int64
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseInt(field, 10, 64)
		if parseErr == nil {
			total += value
		}
	}
	return total, true
}

func hostCPUPct(ctx context.Context, pid int) (float64, bool) {
	processBefore, processOK := procJiffies(pid)
	totalBefore, totalOK := totalJiffies()
	if !processOK || !totalOK {
		return 0, false
	}
	timer := time.NewTimer(cpuSample)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, false
	case <-timer.C:
	}
	processAfter, _ := procJiffies(pid)
	totalAfter, _ := totalJiffies()
	processDelta := processAfter - processBefore
	totalDelta := totalAfter - totalBefore
	if totalDelta <= 0 {
		return 0, true
	}
	percent := 100 * float64(processDelta) / float64(totalDelta) * float64(runtime.NumCPU())
	return math.Round(percent*10) / 10, true
}

func hostRSSMB(pid int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			kb, _ := strconv.Atoi(fields[1])
			return kb / 1024, true
		}
	}
	return 0, false
}

type ioStats struct {
	diskRead  uint64
	diskWrite uint64
	netRX     uint64
	netTX     uint64
}

func processIO(pid int) (ioStats, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return ioStats{}, false
	}
	values := make(map[string]uint64)
	for line := range strings.SplitSeq(string(data), "\n") {
		key, raw, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value, parseErr := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if parseErr == nil {
			values[key] = value
		}
	}
	readBytes, readOK := values["read_bytes"]
	writeBytes, writeOK := values["write_bytes"]
	if !readOK || !writeOK {
		return ioStats{}, false
	}
	stats := ioStats{diskRead: readBytes, diskWrite: writeBytes}
	if rchar, ok := values["rchar"]; ok && rchar > readBytes {
		stats.netRX = rchar - readBytes
	}
	if wchar, ok := values["wchar"]; ok && wchar > writeBytes {
		stats.netTX = wchar - writeBytes
	}
	return stats, true
}

func processUptime(pid int) (int64, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return 0, false
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) <= 19 {
		return 0, false
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	var uptime float64
	if _, err = fmt.Sscanf(string(uptimeData), "%f", &uptime); err != nil {
		return 0, false
	}
	const clockTicks = 100
	seconds := int64(uptime) - int64(startTicks/clockTicks)
	return max(seconds, 0), true
}

func cpusAllowedList(pid, tid int) (string, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/status", pid, tid))
	if err != nil {
		return "", false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:")), true
		}
	}
	return "", false
}

func allPinned(pid int, tids []int) bool {
	if len(tids) == 0 {
		return false
	}
	for _, tid := range tids {
		allowed, ok := cpusAllowedList(pid, tid)
		if !ok || allowed == "" || strings.ContainsAny(allowed, ",- ") {
			return false
		}
	}
	return true
}
