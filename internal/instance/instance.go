// Package instance forwards launches to the already running Ryoport window.
package instance

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const maxMessage = 64 << 10

// Message is one later launch delivered to the running window.
type Message struct {
	Args  []string `json:"args"`
	Token string   `json:"token"`
}

// Send hands m to a running instance. It returns false when no listener owns
// the socket.
func Send(m Message) (bool, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return false, err
	}
	if len(data)+1 > maxMessage {
		return false, fmt.Errorf("instance message exceeds %d bytes", maxMessage)
	}
	conn, err := net.DialTimeout("unix", xdg.Socket(), 250*time.Millisecond)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return false, nil
		}
		var op *net.OpError
		if errors.As(err, &op) && (errors.Is(op.Err, os.ErrNotExist) || errors.Is(op.Err, syscall.ECONNREFUSED)) {
			return false, nil
		}
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return false, err
	}
	var ack [1]byte
	if _, err := io.ReadFull(conn, ack[:]); err != nil {
		return false, err
	}
	if ack[0] != 1 {
		return false, errors.New("running instance rejected the message")
	}
	return true, nil
}

// Listen owns the per-user socket until ctx ends and publishes accepted
// newline-delimited JSON messages.
func Listen(ctx context.Context) (<-chan Message, error) {
	path := xdg.Socket()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); err == nil {
		probe, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			_ = probe.Close()
			return nil, errors.New("another Ryoport instance is already listening")
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale instance socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, err
	}
	messages := make(chan Message, 16)
	go serve(ctx, listener, path, messages)
	return messages, nil
}

func serve(ctx context.Context, listener net.Listener, path string, messages chan Message) {
	var handlers sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		_ = os.Remove(path)
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				break
			}
			continue
		}
		handlers.Add(1)
		go func() {
			defer handlers.Done()
			readMessage(ctx, conn, messages)
		}()
	}
	handlers.Wait()
	close(messages)
	_ = os.Remove(path)
}

func readMessage(ctx context.Context, conn net.Conn, messages chan<- Message) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReaderSize(io.LimitReader(conn, maxMessage+1), maxMessage+1)
	line, err := reader.ReadBytes('\n')
	if err != nil || len(line) > maxMessage || len(line) == 0 {
		_, _ = conn.Write([]byte{0})
		return
	}
	line = line[:len(line)-1]
	var message Message
	if json.Unmarshal(line, &message) != nil {
		_, _ = conn.Write([]byte{0})
		return
	}
	select {
	case messages <- message:
		_, _ = conn.Write([]byte{1})
	case <-ctx.Done():
		_, _ = conn.Write([]byte{0})
	}
}
