package instance

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runtimeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	return dir
}

func TestSendToListener(t *testing.T) {
	dir := runtimeDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	messages, err := Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "ryoport.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %04o, want 0600", info.Mode().Perm())
	}
	want := Message{Args: []string{"--page", "activity"}, Token: "launch-token"}
	done := make(chan struct{})
	var sent bool
	var sendErr error
	go func() {
		sent, sendErr = Send(want)
		close(done)
	}()
	select {
	case got := <-messages:
		if strings.Join(got.Args, "\x00") != strings.Join(want.Args, "\x00") || got.Token != want.Token {
			t.Fatalf("message = %#v, want %#v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("message was not delivered")
	}
	<-done
	if sendErr != nil || !sent {
		t.Fatalf("Send = %v, %v", sent, sendErr)
	}
	cancel()
}

func TestListenRecoversStaleSocket(t *testing.T) {
	dir := runtimeDir(t)
	path := filepath.Join(dir, "ryoport.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	messages, err := Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range messages {
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket remains after cancellation: %v", err)
	}
}

func TestOversizedMessageRejected(t *testing.T) {
	runtimeDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sent, err := Send(Message{Args: []string{strings.Repeat("x", maxMessage)}}); err == nil || sent {
		t.Fatalf("Send oversized = %v, %v", sent, err)
	}
	select {
	case message := <-messages:
		t.Fatalf("listener accepted oversized message: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestListenerRejectsOversizedWireMessage(t *testing.T) {
	dir := runtimeDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := Listen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", filepath.Join(dir, "ryoport.sock"))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Message{Args: []string{strings.Repeat("x", maxMessage)}})
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	select {
	case message := <-messages:
		t.Fatalf("listener accepted oversized wire message: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}
}
