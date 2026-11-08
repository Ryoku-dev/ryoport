// Package fetch downloads files with bounded parallel HTTP ranges.
package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	workers   = 6
	minSize   = 8 << 20
	userAgent = "ryovm-fetch/1"
	tickEvery = 250 * time.Millisecond
)

// Progress is a point-in-time download measurement.
type Progress struct {
	Recv  int64
	Total int64
	BPS   float64
}

// Event is the JSON-lines protocol used by ryovm-fetch.
type Event struct {
	Event    string  `json:"event"`
	Total    int64   `json:"total,omitempty"`
	Parallel bool    `json:"parallel,omitempty"`
	Recv     int64   `json:"recv,omitempty"`
	BPS      float64 `json:"bps,omitempty"`
	Path     string  `json:"path,omitempty"`
	Message  string  `json:"message,omitempty"`
}

type hooks struct {
	start    func(total int64, parallel bool)
	progress func(Progress)
}

// Download fetches url to dest and removes dest unless the complete file is synced.
func Download(ctx context.Context, url, dest string, onProgress func(Progress)) error {
	return DownloadClient(ctx, http.DefaultClient, url, dest, onProgress)
}

func (e Event) MarshalJSON() ([]byte, error) {
	switch e.Event {
	case "start":
		return json.Marshal(struct {
			Event    string `json:"event"`
			Total    int64  `json:"total"`
			Parallel bool   `json:"parallel"`
		}{e.Event, e.Total, e.Parallel})
	case "progress":
		return json.Marshal(struct {
			Event string  `json:"event"`
			Recv  int64   `json:"recv"`
			Total int64   `json:"total"`
			BPS   float64 `json:"bps"`
		}{e.Event, e.Recv, e.Total, e.BPS})
	case "done":
		return json.Marshal(struct {
			Event string `json:"event"`
			Path  string `json:"path"`
		}{e.Event, e.Path})
	case "error":
		return json.Marshal(struct {
			Event   string `json:"event"`
			Message string `json:"message"`
		}{e.Event, e.Message})
	default:
		type plain Event
		return json.Marshal(plain(e))
	}
}

// DownloadClient is Download with an explicit HTTP client.
func DownloadClient(ctx context.Context, client *http.Client, url, dest string, onProgress func(Progress)) error {
	return download(ctx, client, url, dest, hooks{progress: onProgress})
}

func download(ctx context.Context, client *http.Client, url, dest string, h hooks) (err error) {
	if client == nil {
		client = http.DefaultClient
	}
	size, ranged := probe(ctx, client, url)
	parallel := ranged && size >= minSize
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	if h.start != nil {
		h.start(size, parallel)
	}
	ok := false
	defer func() {
		closeErr := out.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
		if !ok || err != nil {
			_ = os.Remove(dest)
		}
	}()

	var recv atomic.Int64
	done := make(chan struct{})
	reportDone := make(chan struct{})
	if h.progress != nil {
		go func() {
			defer close(reportDone)
			report(ctx, &recv, size, done, h.progress)
		}()
	} else {
		close(reportDone)
	}
	if parallel {
		err = downloadParallel(ctx, client, url, out, size, &recv)
	} else {
		err = downloadSingle(ctx, client, url, out, &recv)
	}
	close(done)
	<-reportDone
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if err = out.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if h.progress != nil {
		h.progress(Progress{Recv: recv.Load(), Total: size})
	}
	ok = true
	return nil
}

func probe(ctx context.Context, client *http.Client, url string) (int64, bool) {
	req, err := request(ctx, http.MethodHead, url)
	if err != nil {
		return 0, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}
	return resp.ContentLength, resp.Header.Get("Accept-Ranges") == "bytes" && resp.ContentLength > 0
}

func request(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err == nil {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, err
}

func report(ctx context.Context, recv *atomic.Int64, total int64, done <-chan struct{}, fn func(Progress)) {
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	last := int64(0)
	lastAt := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case now := <-ticker.C:
			cur := recv.Load()
			delta := now.Sub(lastAt).Seconds()
			bps := float64(0)
			if delta > 0 {
				bps = float64(cur-last) / delta
			}
			fn(Progress{Recv: cur, Total: total, BPS: bps})
			last, lastAt = cur, now
		}
	}
}

func downloadSingle(ctx context.Context, client *http.Client, url string, out *os.File, recv *atomic.Int64) error {
	req, err := request(ctx, http.MethodGet, url)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server replied %s", resp.Status)
	}
	_, err = io.Copy(io.MultiWriter(out, countWriter{recv}), resp.Body)
	return err
}

type countWriter struct{ n *atomic.Int64 }

func (w countWriter) Write(p []byte) (int, error) {
	w.n.Add(int64(len(p)))
	return len(p), nil
}

func downloadParallel(ctx context.Context, client *http.Client, url string, out *os.File, size int64, recv *atomic.Int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	span := size / workers
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		start := int64(i) * span
		end := start + span - 1
		if i == workers-1 {
			end = size - 1
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fetchRange(ctx, client, url, out, start, end, recv); err != nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}()
	}
	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		if !errors.Is(err, context.Canceled) || len(errs) == 0 {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func fetchRange(ctx context.Context, client *http.Client, url string, out *os.File, start, end int64, recv *atomic.Int64) error {
	req, err := request(ctx, http.MethodGet, url)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("range request rejected (%s)", resp.Status)
	}
	want := end - start + 1
	written, err := io.Copy(io.NewOffsetWriter(out, start), io.TeeReader(io.LimitReader(resp.Body, want+1), countWriter{recv}))
	if err != nil {
		return err
	}
	if written != want {
		return fmt.Errorf("range %d-%d returned %d bytes", start, end, written)
	}
	return nil
}

// Stream downloads a file and emits the legacy JSON-lines protocol.
func Stream(ctx context.Context, url, dest string, w io.Writer) error {
	enc := json.NewEncoder(w)
	var writeErr error
	emit := func(e Event) {
		if writeErr == nil {
			writeErr = enc.Encode(e)
		}
	}
	err := download(ctx, http.DefaultClient, url, dest, hooks{
		start: func(total int64, parallel bool) {
			emit(Event{Event: "start", Total: total, Parallel: parallel})
		},
		progress: func(p Progress) {
			emit(Event{Event: "progress", Recv: p.Recv, Total: p.Total, BPS: p.BPS})
		},
	})
	if err == nil && writeErr != nil {
		err = writeErr
	}
	if err != nil {
		message := err.Error()
		if errors.Is(err, context.Canceled) {
			message = "cancelled"
		}
		emit(Event{Event: "error", Message: message})
		return err
	}
	emit(Event{Event: "done", Path: dest})
	return writeErr
}
