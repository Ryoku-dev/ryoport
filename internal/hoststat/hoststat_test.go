package hoststat

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseCPUStat(t *testing.T) {
	fixture := `cpu  100 20 30 400 50 6 7 8 900 100
cpu0 40 10 15 200 20 3 4 4 450 50
cpu1 60 10 15 200 30 3 3 4 450 50
intr 1
`
	got, err := ParseCPUStat(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 621 || got.Idle != 450 || got.Cores != 2 {
		t.Fatalf("ParseCPUStat() = %#v, want total 621, idle 450, 2 cores", got)
	}
}

func TestCPUFractionUsesCounterDelta(t *testing.T) {
	previous := CPUCounters{Total: 1000, Idle: 700, Cores: 8}
	current := CPUCounters{Total: 1200, Idle: 820, Cores: 8}
	got, ok := CPUFraction(previous, current)
	if !ok || math.Abs(got-0.4) > 0.000001 {
		t.Fatalf("CPUFraction() = (%v, %v), want (0.4, true)", got, ok)
	}
	if _, ok := CPUFraction(current, previous); ok {
		t.Fatal("CPUFraction accepted counters moving backwards")
	}
}

func TestParseMeminfo(t *testing.T) {
	fixture := `MemTotal:       16384000 kB
MemFree:         1200000 kB
MemAvailable:    6144000 kB
Buffers:          100000 kB
`
	got, err := ParseMeminfo(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 16384000*1024 || got.Available != 6144000*1024 {
		t.Fatalf("ParseMeminfo() = %#v", got)
	}
}

func TestParseMeminfoRequiresAvailableMemory(t *testing.T) {
	if _, err := ParseMeminfo("MemTotal: 1024 kB\nMemFree: 512 kB\n"); err == nil {
		t.Fatal("ParseMeminfo accepted text without MemAvailable")
	}
}

func TestFilesystemUsesNearestExistingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-made", "vms")
	free, total, err := Filesystem(path)
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 || free > total {
		t.Fatalf("Filesystem() = free %d, total %d", free, total)
	}
}
