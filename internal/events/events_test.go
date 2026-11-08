package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/model"
)

func TestLogAppendRecentAndFor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "events.jsonl")
	log, err := Open(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	machine := model.NewID(model.KindMachine, "lab")
	server := model.NewID(model.KindServer, "ally")
	for i, target := range []model.ID{machine, server, machine, machine, server, machine, server} {
		event := model.Event{At: time.Unix(int64(i), 0).UTC(), Target: target, Level: model.EventInfo, Text: string(rune('a' + i))}
		if err := log.Append(event); err != nil {
			t.Fatal(err)
		}
	}
	recent := log.Recent(2)
	if len(recent) != 2 || recent[0].Text != "g" || recent[1].Text != "f" {
		t.Fatalf("unexpected recent events: %#v", recent)
	}
	forMachine := log.For(machine, 3)
	if len(forMachine) != 1 || forMachine[0].Text != "f" {
		t.Fatalf("unexpected machine events: %#v", forMachine)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines > 6 {
		t.Fatalf("event file was not trimmed: %d lines", lines)
	}
}

func TestOpenKeepsLastEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	data := ""
	for i := range 5 {
		data += `{"at":"2026-11-08T00:00:00Z","level":"info","text":"` + string(rune('0'+i)) + `"}` + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	log, err := Open(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := log.Recent(5)
	if len(got) != 2 || got[0].Text != "4" || got[1].Text != "3" {
		t.Fatalf("unexpected retained events: %#v", got)
	}
}

func TestOpenSkipsMalformedLinesAndCleansOnTrim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	first := `{"at":"2026-11-08T00:00:00Z","level":"info","text":"first"}`
	second := `{"at":"2026-11-08T00:00:01Z","level":"info","text":"second"}`
	data := first + "\nnot json\n" + second + "\n" + `{"at":"2026-11-08`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	log, err := Open(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := log.Recent(5)
	if len(got) != 2 || got[0].Text != "second" || got[1].Text != "first" {
		t.Fatalf("unexpected valid events: %#v", got)
	}

	if err := log.Append(model.Event{At: time.Unix(2, 0).UTC(), Level: model.EventInfo, Text: "third"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var texts []string
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("trimmed log contains malformed line %q: %v", scanner.Text(), err)
		}
		texts = append(texts, event.Text)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(texts) != 2 || texts[0] != "second" || texts[1] != "third" {
		t.Fatalf("unexpected trimmed events: %q", texts)
	}
}

func TestLogConcurrentAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	log, err := Open(path, 64)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 16)
	for i := range 16 {
		go func() {
			done <- log.Append(model.Event{At: time.Unix(int64(i), 0), Level: model.EventOK, Text: "ok"})
		}()
	}
	for range 16 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if got := len(log.Recent(64)); got != 16 {
		t.Fatalf("event count = %d", got)
	}
}
