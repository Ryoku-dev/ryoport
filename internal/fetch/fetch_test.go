package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloadUsesSixRanges(t *testing.T) {
	data := bytes.Repeat([]byte("ryoport!"), (minSize/8)+257)
	var ranges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != userAgent {
			t.Errorf("User-Agent = %q", r.UserAgent())
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		start, end, ok := parseTestRange(r.Header.Get("Range"))
		if !ok || start < 0 || end >= int64(len(data)) || end < start {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		ranges.Add(1)
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[start : end+1])
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "image.qcow2")
	var last Progress
	if err := Download(context.Background(), server.URL, dest, func(p Progress) { last = p }); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes differ")
	}
	if ranges.Load() != workers {
		t.Fatalf("range requests = %d, want %d", ranges.Load(), workers)
	}
	if last.Recv != int64(len(data)) || last.Total != int64(len(data)) {
		t.Fatalf("final progress = %+v", last)
	}
}

func TestDownloadCancellationRemovesDestination(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "0")
			return
		}
		close(started)
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	dest := filepath.Join(t.TempDir(), "partial.img")
	done := make(chan error, 1)
	go func() { done <- Download(ctx, server.URL, dest, nil) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Download returned nil after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Download did not stop")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("partial destination remains: %v", err)
	}
}

func TestStreamLegacyJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "3")
			return
		}
		_, _ = io.WriteString(w, "abc")
	}))
	defer server.Close()
	var output bytes.Buffer
	dest := filepath.Join(t.TempDir(), "small.iso")
	if err := Stream(context.Background(), server.URL, dest, &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if lines[0] != `{"event":"start","total":3,"parallel":false}` {
		t.Fatalf("start event = %s", lines[0])
	}
	if lines[len(lines)-1] != `{"event":"done","path":"`+dest+`"}` {
		t.Fatalf("done event = %s", lines[len(lines)-1])
	}
}

func parseTestRange(value string) (int64, int64, bool) {
	value = strings.TrimPrefix(value, "bytes=")
	left, right, ok := strings.Cut(value, "-")
	if !ok {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(left, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.ParseInt(right, 10, 64)
	return start, end, err == nil
}
