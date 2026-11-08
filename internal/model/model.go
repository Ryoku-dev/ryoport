// Package model holds the types every other package shares: what a resource
// is, the state it is in, and the jobs, events and alerts around it.
package model

import (
	"strings"
	"time"
)

type Kind string

const (
	KindMachine     Kind = "machine"
	KindPassthrough Kind = "passthrough"
	KindServer      Kind = "server"
	KindConsole     Kind = "console"
)

// ID is "<kind>:<key>", for example "machine:ubuntu-24.04" or "server:ally".
// The key is the machine name, the SSH alias or the console profile id.
type ID string

func NewID(k Kind, key string) ID { return ID(string(k) + ":" + key) }

func (id ID) Kind() Kind {
	k, _, _ := strings.Cut(string(id), ":")
	return Kind(k)
}

func (id ID) Key() string {
	_, key, _ := strings.Cut(string(id), ":")
	return key
}

type State string

const (
	StateUnknown    State = "unknown"
	StateStopped    State = "stopped"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StatePaused     State = "paused"
	StateStopping   State = "stopping"
	StateCreating   State = "creating"
	StateAbsent     State = "absent"
	StateOnline     State = "online"
	StateOffline    State = "offline"
	StateConnecting State = "connecting"
	StateNeedsAuth  State = "needs-auth"
	StateDegraded   State = "degraded"
	StateError      State = "error"
)

// Tone is the family a state belongs to. The seal's shape and ink come from
// the tone, never from the state directly, so a new state only has to pick one.
type Tone int

const (
	ToneIdle Tone = iota
	ToneAlive
	ToneBusy
	ToneAttention
	ToneBad
	TonePaused
)

func (s State) Tone() Tone {
	switch s {
	case StateRunning, StateOnline:
		return ToneAlive
	case StateStarting, StateStopping, StateCreating, StateConnecting:
		return ToneBusy
	case StateNeedsAuth, StateDegraded:
		return ToneAttention
	case StateError:
		return ToneBad
	case StatePaused:
		return TonePaused
	}
	return ToneIdle
}

func (s State) Label() string {
	switch s {
	case StateStopped:
		return "Stopped"
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StatePaused:
		return "Paused"
	case StateStopping:
		return "Stopping"
	case StateCreating:
		return "Creating"
	case StateAbsent:
		return "Missing"
	case StateOnline:
		return "Online"
	case StateOffline:
		return "Offline"
	case StateConnecting:
		return "Connecting"
	case StateNeedsAuth:
		return "Needs password"
	case StateDegraded:
		return "Degraded"
	case StateError:
		return "Failed"
	}
	return "Unknown"
}

// Verb is the one action a row offers.
type Verb string

const (
	VerbNone    Verb = ""
	VerbStart   Verb = "start"
	VerbStop    Verb = "stop"
	VerbOpen    Verb = "open"
	VerbConnect Verb = "connect"
	VerbSetPass Verb = "setpass"
)

// Resource is one row of the fleet list, built by the store from whatever
// backend owns the thing.
type Resource struct {
	ID      ID
	Kind    Kind
	Name    string
	State   State
	Detail  string // the row's one line of facts
	Icon    string // an OS slug for machines ("ubuntu"), otherwise "server", "serial", "telnet", "rdp", "vnc"
	Group   string
	Tags    []string
	Pinned  bool
	Primary Verb
	Updated time.Time
}

type Metric string

const (
	MetricCPU  Metric = "cpu"  // 0..1
	MetricMem  Metric = "mem"  // 0..1
	MetricDisk Metric = "disk" // bytes per second, read and write together
	MetricNet  Metric = "net"  // bytes per second, in and out together
)

// Series is a fixed-size ring of samples, oldest first.
type Series struct {
	vals []float64
	at   []time.Time
	head int
	n    int
}

func NewSeries(size int) *Series {
	if size < 1 {
		size = 1
	}
	return &Series{vals: make([]float64, size), at: make([]time.Time, size)}
}

func (s *Series) Push(v float64, t time.Time) {
	s.vals[s.head] = v
	s.at[s.head] = t
	s.head = (s.head + 1) % len(s.vals)
	if s.n < len(s.vals) {
		s.n++
	}
}

func (s *Series) Len() int { return s.n }

// Values returns the samples oldest first in a new slice.
func (s *Series) Values() []float64 {
	out := make([]float64, s.n)
	start := (s.head - s.n + len(s.vals)) % len(s.vals)
	for i := range s.n {
		out[i] = s.vals[(start+i)%len(s.vals)]
	}
	return out
}

func (s *Series) Last() (float64, time.Time, bool) {
	if s.n == 0 {
		return 0, time.Time{}, false
	}
	i := (s.head - 1 + len(s.vals)) % len(s.vals)
	return s.vals[i], s.at[i], true
}

// Clone copies the ring so a snapshot can hand it to a frame safely.
func (s *Series) Clone() *Series {
	return &Series{vals: append([]float64(nil), s.vals...), at: append([]time.Time(nil), s.at...), head: s.head, n: s.n}
}

type JobState string

const (
	JobRunning   JobState = "running"
	JobDone      JobState = "done"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)

// Job is a long action: a download, a build, a start that takes a while.
type Job struct {
	ID         string
	Target     ID
	Title      string  // "Ubuntu 24.04"
	Phase      string  // "Downloading", "Writing config"
	Progress   float64 // 0..1, or -1 when unknown
	Done       int64   // bytes, when the job moves bytes
	Total      int64
	Rate       float64 // bytes per second
	Started    time.Time
	Finished   time.Time
	State      JobState
	Err        string
	Cancelable bool
}

type EventLevel string

const (
	EventInfo  EventLevel = "info"
	EventOK    EventLevel = "ok"
	EventWarn  EventLevel = "warn"
	EventError EventLevel = "error"
)

type Event struct {
	At     time.Time  `json:"at"`
	Target ID         `json:"target,omitempty"`
	Level  EventLevel `json:"level"`
	Text   string     `json:"text"`
	Detail string     `json:"detail,omitempty"`
}

// Fix names the remedy an alert offers next to itself.
type Fix string

const (
	FixNone          Fix = ""
	FixSavePassword  Fix = "save-password"
	FixDeployKey     Fix = "deploy-key"
	FixInstallEngine Fix = "install-engine"
	FixEnableKVM     Fix = "enable-kvm"
	FixOpenHubGPU    Fix = "open-hub-gpu"
	FixOpenSettings  Fix = "open-settings"
)

type Severity string

const (
	SeverityWarn Severity = "warn"
	SeverityBad  Severity = "bad"
)

// Alert is something that needs the user. The store derives alerts from the
// latest state on every change, so they clear themselves once fixed.
type Alert struct {
	ID       string
	Target   ID
	Severity Severity
	Title    string
	Body     string
	Fix      Fix
}
