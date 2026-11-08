package passthrough

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCreateValidationMatchesUILimits(t *testing.T) {
	iso := filepath.Join(t.TempDir(), "guest.iso")
	if err := os.WriteFile(iso, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	valid := CreateOptions{Name: "guest_1", ISO: iso, OS: "linux", DiskGB: 16, RAMMB: 2048}
	if err := validateCreate(valid); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	for _, mutate := range []func(*CreateOptions){
		func(options *CreateOptions) { options.Name = "bad name" },
		func(options *CreateOptions) { options.OS = "bsd" },
		func(options *CreateOptions) { options.DiskGB = 15 },
		func(options *CreateOptions) { options.DiskGB = 1025 },
		func(options *CreateOptions) { options.RAMMB = 1024 },
		func(options *CreateOptions) { options.RAMMB = 129 * 1024 },
		func(options *CreateOptions) { options.RAMMB = 2500 },
	} {
		options := valid
		mutate(&options)
		if err := validateCreate(options); err == nil {
			t.Fatalf("invalid options accepted: %+v", options)
		}
	}
}

func TestReadStatsMatchesLegacyJSONCounters(t *testing.T) {
	old := commandOutput
	oldLookup := lookupCommand
	defer func() { lookupCommand = oldLookup }()
	lookupCommand = func(string) (string, error) { return "virsh", nil }
	defer func() { commandOutput = old }()
	commandOutput = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if reflect.DeepEqual(args[2:], []string{"domstate", "ryoku-win11"}) {
			return []byte("running\n"), nil
		}
		if len(args) > 2 && args[2] == "domstats" {
			return []byte("cpu.time=90\nballoon.current=1024\nblock.0.rd.bytes=10\nblock.1.rd.bytes=20\nblock.0.wr.bytes=3\nnet.0.rx.bytes=7\nnet.0.tx.bytes=8\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	stats, err := ReadStats(context.Background(), "win11")
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Running || stats.CPUTimeNS != 90 || stats.MemoryKB != 1024 || stats.DiskReadBytes != 30 || stats.DiskWriteBytes != 3 || stats.NetReceiveBytes != 7 || stats.NetSendBytes != 8 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
