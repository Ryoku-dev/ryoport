// Package format turns numbers and times into the short words the window
// shows: sizes, rates, durations and how long ago something happened.
package format

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

var units = [...]string{"B", "KB", "MB", "GB", "TB", "PB"}

// Bytes uses powers of 1024 with the labels people read on disks and RAM.
func Bytes(n int64) string {
	if n < 0 {
		n = 0
	}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return number(v, i) + " " + units[i]
}

func Rate(bytesPerSecond float64) string {
	if bytesPerSecond < 0 || math.IsNaN(bytesPerSecond) {
		bytesPerSecond = 0
	}
	return Bytes(int64(bytesPerSecond)) + "/s"
}

// Progress reads "1.4 of 2.3 GB", naming the unit once.
func Progress(done, total int64) string {
	if total <= 0 {
		return Bytes(done)
	}
	t := float64(total)
	i := 0
	for t >= 1024 && i < len(units)-1 {
		t /= 1024
		i++
	}
	d := float64(done) / math.Pow(1024, float64(i))
	return number(d, i) + " of " + number(t, i) + " " + units[i]
}

func number(v float64, unit int) string {
	if unit == 0 || v >= 100 {
		return strconv.FormatFloat(math.Round(v), 'f', 0, 64)
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

func Percent(frac float64) string {
	if math.IsNaN(frac) {
		return "0%"
	}
	return strconv.Itoa(int(math.Round(max(0, min(1, frac))*100))) + "%"
}

// Duration is for lengths: "40 s", "3 min", "2 h 5 min", "12 days".
func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Round(time.Second)/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Round(time.Minute)/time.Minute))
	case d < 24*time.Hour:
		h := int(d / time.Hour)
		m := int((d % time.Hour).Round(time.Minute) / time.Minute)
		if m == 60 {
			h, m = h+1, 0
		}
		if m == 0 {
			return fmt.Sprintf("%d h", h)
		}
		return fmt.Sprintf("%d h %d min", h, m)
	}
	days := int(d / (24 * time.Hour))
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// Left is for what remains: "about 3 min left", "less than a minute left".
func Left(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute left"
	}
	return "about " + Duration(d) + " left"
}

// Ago reads "just now", "2 min ago", "yesterday", or a date past a week.
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < 10*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%d s ago", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d/time.Hour))
	}
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.AddDate(0, 0, -1).Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return "yesterday"
	}
	if d < 7*24*time.Hour {
		return t.Format("Monday")
	}
	if t.Year() == now.Year() {
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

func Latency(ms int) string {
	if ms < 1 {
		return "under 1 ms"
	}
	return strconv.Itoa(ms) + " ms"
}
