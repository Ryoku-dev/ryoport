package common

import (
	"math"
	"strconv"
	"time"

	"gioui.org/widget"

	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/env"
	"github.com/ryoku-dev/ryoport/ui/format"
	"github.com/ryoku-dev/ryoport/ui/icons"
)

func StateWords(e *env.Env, r model.Resource) (claim, proof string) {
	if e == nil || e.Snap == nil {
		return r.State.Label(), ""
	}
	s := e.Snap
	switch r.Kind {
	case model.KindServer:
		server := s.Servers[r.ID.Key()]
		switch r.State {
		case model.StateOnline:
			if server != nil && server.Reach != nil {
				return "Online", format.Latency(server.Reach.RTTMs)
			}
			return "Online", ""
		case model.StateNeedsAuth, model.StateDegraded:
			return "Needs you", format.Ago(r.Updated, e.Now)
		default:
			return "Offline", ""
		}
	case model.KindMachine:
		if job, ok := s.JobFor(r.ID); ok {
			claim = r.State.Label()
			if job.Progress >= 0 {
				claim = format.Percent(job.Progress)
			}
			return claim, jobTimeLeft(job, e.Now)
		}
		switch r.State {
		case model.StateRunning:
			machine := s.Machines[r.ID.Key()]
			if machine != nil && machine.Stats != nil {
				if machine.Stats.HostCPUPct != nil {
					return "Running", wholePercent(*machine.Stats.HostCPUPct)
				}
				if machine.Stats.UptimeS != nil {
					return "Running", format.Duration(time.Duration(*machine.Stats.UptimeS) * time.Second)
				}
			}
			return "Running", ""
		case model.StateStopped:
			return "Stopped", ""
		case model.StateAbsent:
			return "Missing", ""
		default:
			return r.State.Label(), ""
		}
	case model.KindConsole:
		if r.State == model.StateOnline {
			return "Ready", ""
		}
		return "Offline", ""
	case model.KindPassthrough:
		if r.State == model.StateRunning {
			return "Running", ""
		}
		return "Stopped", ""
	default:
		return r.State.Label(), ""
	}
}

func jobTimeLeft(job model.Job, now time.Time) string {
	var left time.Duration
	if job.Rate > 0 && job.Total > job.Done {
		left = time.Duration(float64(time.Second) * float64(job.Total-job.Done) / job.Rate)
	} else if job.Progress > 0 && job.Progress < 1 && !job.Started.IsZero() && now.After(job.Started) {
		left = time.Duration(float64(now.Sub(job.Started)) * (1 - job.Progress) / job.Progress)
	}
	if left <= 0 {
		return ""
	}
	return format.Left(left)
}

func wholePercent(value float64) string {
	if math.IsNaN(value) || value < 0 {
		value = 0
	}
	return strconv.Itoa(int(math.Round(value))) + "%"
}

func PrimaryLabel(r model.Resource) (label string, icon *widget.Icon) {
	switch r.Primary {
	case model.VerbConnect:
		return "Connect", icons.Terminal
	case model.VerbStart:
		return "Start", icons.PlayArrow
	case model.VerbOpen:
		return "Open", icons.OpenInNew
	case model.VerbSetPass:
		return "Enter password", icons.Key
	case model.VerbStop:
		return "Stop", icons.Stop
	default:
		return "", nil
	}
}

func FleetSentence(s *store.Snapshot) (spans []Span, attention int) {
	if s == nil || len(s.Resources) == 0 {
		return []Span{{Text: "Nothing here yet."}}, -1
	}
	running := 0
	for _, r := range s.Resources {
		switch r.Kind {
		case model.KindMachine, model.KindPassthrough:
			if r.State == model.StateRunning {
				running++
			}
		case model.KindServer, model.KindConsole:
			if r.State == model.StateOnline {
				running++
			}
		}
	}
	name := attentionName(s)
	if name == "" && running == len(s.Resources) {
		return []Span{{Text: "Everything is up."}}, -1
	}
	if running == 0 {
		spans = append(spans, Span{Text: "Nothing is running."})
	} else {
		spans = append(spans, Span{Text: countWord(running) + " running."})
	}
	if name == "" {
		return spans, -1
	}
	spans[0].Text += " "
	attention = len(spans)
	spans = append(spans, Span{Text: name}, Span{Text: " needs you."})
	return spans, attention
}

func attentionName(s *store.Snapshot) string {
	for _, alert := range s.Alerts {
		if alert.Target == "" {
			continue
		}
		if resource, ok := s.Resource(alert.Target); ok && resource.Name != "" {
			return resource.Name
		}
	}
	for _, r := range s.Resources {
		if r.State.Tone() == model.ToneAttention || r.State.Tone() == model.ToneBad {
			return r.Name
		}
	}
	return ""
}

func countWord(n int) string {
	words := [...]string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve"}
	if n > 0 && n < len(words) {
		return words[n]
	}
	return strconv.Itoa(n)
}
