package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type qemuImageInfo struct {
	Snapshots []struct {
		Name    string      `json:"name"`
		DateSec json.Number `json:"date-sec"`
	} `json:"snapshots"`
}

// Snapshots lists qcow2 snapshots newest first.
func (m *Manager) Snapshots(ctx context.Context, name string) ([]Snapshot, error) {
	disk, err := m.DiskPath(name)
	if err != nil {
		return nil, err
	}
	if !pathExists(disk) {
		return []Snapshot{}, nil
	}
	output, err := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", disk).Output()
	if err != nil {
		return []Snapshot{}, nil
	}
	var info qemuImageInfo
	if json.Unmarshal(output, &info) != nil {
		return []Snapshot{}, nil
	}
	snapshots := make([]Snapshot, 0, len(info.Snapshots))
	for i := len(info.Snapshots) - 1; i >= 0; i-- {
		seconds, _ := strconv.ParseInt(info.Snapshots[i].DateSec.String(), 10, 64)
		snapshots = append(snapshots, Snapshot{
			Name: info.Snapshots[i].Name,
			Date: time.Unix(seconds, 0).UTC().Format(time.RFC3339),
		})
	}
	return snapshots, nil
}

// CreateSnapshot records the current stopped machine state.
func (m *Manager) CreateSnapshot(ctx context.Context, name, tag string) error {
	return m.snapshotChange(ctx, name, tag, "-c")
}

// RestoreSnapshot restores a stopped machine snapshot.
func (m *Manager) RestoreSnapshot(ctx context.Context, name, tag string) error {
	return m.snapshotChange(ctx, name, tag, "-a")
}

// DeleteSnapshot removes a stopped machine snapshot.
func (m *Manager) DeleteSnapshot(ctx context.Context, name, tag string) error {
	return m.snapshotChange(ctx, name, tag, "-d")
}

func (m *Manager) snapshotChange(ctx context.Context, name, tag, action string) error {
	if tag == "" {
		switch action {
		case "-c":
			return errors.New("create needs a tag")
		case "-a":
			return errors.New("restore needs a tag")
		default:
			return errors.New("delete needs a tag")
		}
	}
	if !qconfValue(tag) {
		return errors.New("snapshot tag contains unsafe characters")
	}
	disk, _, pid, _, err := m.runtimePaths(name)
	if err != nil {
		return err
	}
	if !pathExists(disk) {
		return fmt.Errorf("%s has no disk yet; launch it once first", name)
	}
	if running(pid) {
		return fmt.Errorf("stop %s before managing snapshots", name)
	}
	command := exec.CommandContext(ctx, "qemu-img", "snapshot", action, tag, disk)
	if output, err := command.CombinedOutput(); err != nil {
		if message := firstNonemptyLine(output); message != "" {
			return errors.New(message)
		}
		return err
	}
	return nil
}

// Seal replaces the reserved sealed snapshot with the current state.
func (m *Manager) Seal(ctx context.Context, name string) error {
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	disk, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before sealing it", name)
	}
	if !pathExists(disk) {
		return fmt.Errorf("%s has no disk yet; install it first, then seal", name)
	}
	sealed := false
	for range 12 {
		_ = exec.CommandContext(ctx, "qemu-img", "snapshot", "-d", "sealed", disk).Run()
		if exec.CommandContext(ctx, "qemu-img", "snapshot", "-c", "sealed", disk).Run() == nil {
			sealed = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if !sealed {
		return errors.New("could not write the seal")
	}
	if err := conf.Set("ryovm_sealed", "1"); err != nil {
		return err
	}
	return conf.Write(confPath)
}

// RestoreSeal rolls a stopped machine back to its reserved sealed snapshot.
func (m *Manager) RestoreSeal(ctx context.Context, name string) error {
	disk, _, pid, _, err := m.runtimePaths(name)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before restoring the seal", name)
	}
	if exec.CommandContext(ctx, "qemu-img", "snapshot", "-a", "sealed", disk).Run() != nil {
		return fmt.Errorf("no seal to restore on %s", name)
	}
	return nil
}

func qconfValue(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\x00\n\r")
}

func firstNonemptyLine(data []byte) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line != "" {
			return line
		}
	}
	return ""
}
