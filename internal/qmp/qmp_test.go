package qmp

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestHMPParsers(t *testing.T) {
	reply := "\x1b[2KVM status: paused\r\n(qemu)"
	clean := ansi.ReplaceAllString(reply, "")
	if got := parseStatus(clean); got != "paused" {
		t.Fatalf("parseStatus() = %q", got)
	}
	if got, ok := parseBalloon("balloon: actual=4096"); !ok || got != 4096 {
		t.Fatalf("parseBalloon() = %d, %v", got, ok)
	}
	want := []int{411, 412}
	if got := parseCPUs("CPU #0: thread_id=411\nCPU #1: thread_id=412"); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCPUs() = %v, want %v", got, want)
	}
}

func TestCollectFakeMonitor(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "monitor.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	served := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			served <- err
			return
		}
		defer conn.Close()
		if _, err = conn.Write([]byte("QEMU 9.0 monitor\r\n(qemu)")); err != nil {
			served <- err
			return
		}
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var response string
			switch scanner.Text() {
			case "info status":
				response = "\x1b[2KVM status: running\r\n(qemu)"
			case "info balloon":
				response = "balloon: actual=2048\r\n(qemu)"
			case "info cpus":
				response = "CPU #0: thread_id=101\r\nCPU #1: thread_id=102\r\n(qemu)"
			default:
				response = "unknown command\r\n(qemu)"
			}
			if _, err = conn.Write([]byte(response)); err != nil {
				served <- err
				return
			}
			if scanner.Text() == "info cpus" {
				served <- nil
				return
			}
		}
		served <- scanner.Err()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stats, err := Collect(ctx, socket, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Running || stats.Status != "running" {
		t.Fatalf("unexpected status: %+v", stats)
	}
	if stats.BalloonMB == nil || *stats.BalloonMB != 2048 {
		t.Fatalf("balloon = %v", stats.BalloonMB)
	}
	if stats.VCPUs == nil || *stats.VCPUs != 2 || stats.VCPUTIDs == nil || !reflect.DeepEqual(*stats.VCPUTIDs, []int{101, 102}) {
		t.Fatalf("unexpected cpu data: %+v", stats)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestCollectMissingMonitorCompatibilityJSON(t *testing.T) {
	stats, err := Collect(context.Background(), filepath.Join(t.TempDir(), "missing.sock"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"running":false}` {
		t.Fatalf("JSON = %s", data)
	}
}
