package format

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	for _, c := range []struct {
		in   int64
		want string
	}{
		{-5, "0 B"}, {0, "0 B"}, {1023, "1023 B"}, {1024, "1 KB"}, {1536, "1.5 KB"},
		{8 << 30, "8 GB"}, {469 << 30, "469 GB"}, {1503238553, "1.4 GB"}, {150 << 20, "150 MB"},
	} {
		if got := Bytes(c.in); got != c.want {
			t.Errorf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProgressNamesTheUnitOnce(t *testing.T) {
	if got := Progress(1503238553, 2469606195); got != "1.4 of 2.3 GB" {
		t.Errorf("Progress = %q", got)
	}
	if got := Progress(512, 0); got != "512 B" {
		t.Errorf("Progress without total = %q", got)
	}
}

func TestPercentClamps(t *testing.T) {
	for in, want := range map[float64]string{-1: "0%", 0.618: "62%", 1.4: "100%"} {
		if got := Percent(in); got != want {
			t.Errorf("Percent(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	for _, c := range []struct {
		in   time.Duration
		want string
	}{
		{40 * time.Second, "40 s"}, {3 * time.Minute, "3 min"}, {2*time.Hour + 5*time.Minute, "2 h 5 min"},
		{2*time.Hour + 59*time.Minute + 50*time.Second, "3 h"}, {26 * time.Hour, "1 day"}, {12 * 24 * time.Hour, "12 days"},
	} {
		if got := Duration(c.in); got != c.want {
			t.Errorf("Duration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{time.Time{}, "never"},
		{now.Add(-3 * time.Second), "just now"},
		{now.Add(-2 * time.Minute), "2 min ago"},
		{now.Add(-5 * time.Hour), "5 h ago"},
		{time.Date(2026, 11, 7, 9, 0, 0, 0, time.Local), "yesterday"},
		{time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local), "Wednesday"},
		{time.Date(2026, 9, 1, 9, 0, 0, 0, time.Local), "1 Sep"},
		{time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local), "1 Sep 2025"},
	} {
		if got := Ago(c.at, now); got != c.want {
			t.Errorf("Ago(%v) = %q, want %q", c.at, got, c.want)
		}
	}
}
