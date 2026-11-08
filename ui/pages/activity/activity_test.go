package activity

import (
	"math"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoport/internal/model"
)

func TestGroupEventsByDay(t *testing.T) {
	location := time.FixedZone("test", 2*60*60)
	now := time.Date(2026, time.November, 8, 15, 0, 0, 0, location)
	events := []model.Event{
		{At: time.Date(2026, time.October, 20, 9, 0, 0, 0, location), Text: "old"},
		{At: time.Date(2026, time.November, 7, 23, 0, 0, 0, location), Text: "yesterday"},
		{At: time.Date(2026, time.November, 8, 8, 0, 0, 0, location), Text: "today early"},
		{At: time.Date(2026, time.November, 5, 10, 0, 0, 0, location), Text: "weekday"},
		{At: time.Date(2026, time.November, 8, 14, 0, 0, 0, location), Text: "today late"},
	}

	groups := groupEvents(events, now)
	if len(groups) != 4 {
		t.Fatalf("got %d groups, want 4", len(groups))
	}
	wantLabels := []string{"Today", "Yesterday", "Thursday", "20 October 2026"}
	for index, want := range wantLabels {
		if groups[index].Label != want {
			t.Errorf("group %d label = %q, want %q", index, groups[index].Label, want)
		}
	}
	if got := groups[0].Events[0].Text; got != "today late" {
		t.Errorf("first event = %q, want newest event", got)
	}
}

func TestDayLabelUsesCalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.March, 30, 0, 30, 0, 0, location)
	previous := time.Date(2026, time.March, 29, 23, 30, 0, 0, location)
	if got := dayLabel(previous, now); got != "Yesterday" {
		t.Fatalf("dayLabel = %q, want Yesterday", got)
	}
}

func TestJobTimelineUsesOneClampedHour(t *testing.T) {
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	jobs := []model.Job{
		{ID: "running", Started: now.Add(-30 * time.Minute), State: model.JobRunning},
		{ID: "finished", Started: now.Add(-50 * time.Minute), Finished: now.Add(-20 * time.Minute), State: model.JobDone},
		{ID: "clipped", Started: now.Add(-2 * time.Hour), Finished: now.Add(-10 * time.Minute), State: model.JobDone},
		{ID: "old", Started: now.Add(-2 * time.Hour), Finished: now.Add(-90 * time.Minute), State: model.JobDone},
		{ID: "future", Started: now.Add(time.Minute), State: model.JobRunning},
	}

	spans := jobTimeline(jobs, now)
	if len(spans) != 3 {
		t.Fatalf("got %d spans, want 3", len(spans))
	}
	byID := make(map[string]timelineSpan, len(spans))
	for _, span := range spans {
		byID[span.Job.ID] = span
	}
	assertNear(t, byID["running"].Start, 0.5)
	assertNear(t, byID["running"].End, 1)
	assertNear(t, byID["finished"].Start, 1.0/6.0)
	assertNear(t, byID["finished"].End, 2.0/3.0)
	assertNear(t, byID["clipped"].Start, 0)
	assertNear(t, byID["clipped"].End, 5.0/6.0)
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("got %.12f, want %.12f", got, want)
	}
}
