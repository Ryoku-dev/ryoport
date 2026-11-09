// Package hoststat samples the local computer without keeping a background worker.
package hoststat

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// CPUCounters is one cumulative /proc/stat reading. Idle includes iowait.
type CPUCounters struct {
	Total uint64
	Idle  uint64
	Cores int
}

// Memory is one /proc/meminfo reading, in bytes.
type Memory struct {
	Total     uint64
	Available uint64
}

// Sample is the raw host reading used to derive a CPU delta and publish the
// point-in-time memory and filesystem values.
type Sample struct {
	CPU       CPUCounters
	Memory    Memory
	DiskFree  uint64
	DiskTotal uint64
	At        time.Time
}

// Read samples procfs and the filesystem containing vmDir. If vmDir has not
// been made yet, disk usage comes from its nearest existing parent.
func Read(vmDir string, now time.Time) (Sample, error) {
	statText, err := os.ReadFile("/proc/stat")
	if err != nil {
		return Sample{}, fmt.Errorf("read /proc/stat: %w", err)
	}
	cpu, err := parseCPUStat(statText)
	if err != nil {
		return Sample{}, err
	}
	memText, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Sample{}, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	memory, err := parseMeminfo(memText)
	if err != nil {
		return Sample{}, err
	}
	free, total, err := Filesystem(vmDir)
	if err != nil {
		return Sample{}, err
	}
	return Sample{CPU: cpu, Memory: memory, DiskFree: free, DiskTotal: total, At: now}, nil
}

// ParseCPUStat parses the aggregate counters and logical core count from
// /proc/stat text. Guest counters are excluded because Linux already includes
// them in user and nice time.
func ParseCPUStat(text string) (CPUCounters, error) {
	return parseCPUStat([]byte(text))
}

func parseCPUStat(text []byte) (CPUCounters, error) {
	var result CPUCounters
	found := false
	for len(text) > 0 {
		var line []byte
		line, text = takeLine(text)
		if len(line) >= 4 && line[0] == 'c' && line[1] == 'p' && line[2] == 'u' && (line[3] == ' ' || line[3] == '\t') {
			fields := bytes.Fields(line)
			if len(fields) < 5 {
				return CPUCounters{}, errors.New("parse /proc/stat: aggregate cpu line is incomplete")
			}
			limit := min(len(fields), 9)
			for index := 1; index < limit; index++ {
				value, err := parseUint(fields[index])
				if err != nil {
					return CPUCounters{}, fmt.Errorf("parse /proc/stat cpu field %q: %w", fields[index], err)
				}
				result.Total += value
				if index == 4 || index == 5 {
					result.Idle += value
				}
			}
			found = true
			continue
		}
		end := bytes.IndexAny(line, " \t")
		name := line
		if end >= 0 {
			name = line[:end]
		}
		if isCPUCore(name) {
			result.Cores++
		}
	}
	if !found {
		return CPUCounters{}, errors.New("parse /proc/stat: aggregate cpu line is missing")
	}
	if result.Cores == 0 {
		return CPUCounters{}, errors.New("parse /proc/stat: logical cpu lines are missing")
	}
	return result, nil
}

func isCPUCore(name []byte) bool {
	if len(name) <= 3 || name[0] != 'c' || name[1] != 'p' || name[2] != 'u' {
		return false
	}
	for _, value := range name[3:] {
		if value < '0' || value > '9' {
			return false
		}
	}
	return true
}

// CPUFraction calculates non-idle CPU time between two cumulative readings.
func CPUFraction(previous, current CPUCounters) (float64, bool) {
	if current.Total <= previous.Total || current.Idle < previous.Idle {
		return 0, false
	}
	total := current.Total - previous.Total
	idle := current.Idle - previous.Idle
	if idle >= total {
		return 0, true
	}
	return float64(total-idle) / float64(total), true
}

// ParseMeminfo parses MemTotal and MemAvailable from /proc/meminfo text.
func ParseMeminfo(text string) (Memory, error) {
	return parseMeminfo([]byte(text))
}

func parseMeminfo(text []byte) (Memory, error) {
	var memory Memory
	var haveTotal, haveAvailable bool
	for len(text) > 0 {
		var line []byte
		line, text = takeLine(text)
		totalLine := bytes.HasPrefix(line, []byte("MemTotal:"))
		availableLine := bytes.HasPrefix(line, []byte("MemAvailable:"))
		if !totalLine && !availableLine {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) < 3 || !bytes.Equal(fields[2], []byte("kB")) {
			return Memory{}, fmt.Errorf("parse /proc/meminfo: malformed %s", bytes.TrimSpace(line))
		}
		var destination *uint64
		switch {
		case totalLine:
			destination = &memory.Total
			haveTotal = true
		case availableLine:
			destination = &memory.Available
			haveAvailable = true
		}
		value, err := parseUint(fields[1])
		if err != nil {
			return Memory{}, fmt.Errorf("parse /proc/meminfo %s: %w", fields[0], err)
		}
		if value > ^uint64(0)/1024 {
			return Memory{}, fmt.Errorf("parse /proc/meminfo %s: value overflows bytes", fields[0])
		}
		*destination = value * 1024
	}
	if !haveTotal || !haveAvailable {
		return Memory{}, errors.New("parse /proc/meminfo: MemTotal or MemAvailable is missing")
	}
	if memory.Available > memory.Total {
		return Memory{}, errors.New("parse /proc/meminfo: MemAvailable exceeds MemTotal")
	}
	return memory, nil
}

func takeLine(text []byte) (line, rest []byte) {
	if end := bytes.IndexByte(text, '\n'); end >= 0 {
		return text[:end], text[end+1:]
	}
	return text, nil
}

func parseUint(value []byte) (uint64, error) {
	if len(value) == 0 {
		return 0, errors.New("empty number")
	}
	var result uint64
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("invalid number %q", value)
		}
		number := uint64(digit - '0')
		if result > (^uint64(0)-number)/10 {
			return 0, fmt.Errorf("number overflows uint64: %q", value)
		}
		result = result*10 + number
	}
	return result, nil
}

// Filesystem returns bytes available to this user and total bytes on the
// filesystem containing path.
func Filesystem(path string) (free, total uint64, err error) {
	path = filepath.Clean(path)
	for {
		var stat unix.Statfs_t
		err = unix.Statfs(path, &stat)
		if err == nil {
			blockSize := uint64(stat.Bsize)
			return stat.Bavail * blockSize, stat.Blocks * blockSize, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return 0, 0, fmt.Errorf("stat filesystem for %s: %w", path, err)
		}
		if !errors.Is(err, unix.ENOENT) && !errors.Is(err, unix.ENOTDIR) {
			return 0, 0, fmt.Errorf("stat filesystem for %s: %w", path, err)
		}
		path = parent
	}
}
