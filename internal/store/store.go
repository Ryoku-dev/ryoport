package store

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/events"
	"github.com/ryoku-dev/ryoport/internal/keyring"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

type counterSample struct {
	at      time.Time
	disk    uint64
	net     uint64
	cpuTime uint64
}

type undoEntry struct {
	expires time.Time
	host    *remote.HostInput
	console *console.Profile
	done    <-chan struct{}
}

type runningJob struct {
	job    model.Job
	cancel context.CancelFunc
}

type Store struct {
	o Options

	mu       sync.Mutex
	snapshot atomic.Pointer[Snapshot]
	ctx      context.Context
	started  bool
	focus    Focus
	wakeMu   sync.Mutex
	wake     chan struct{}

	host           Host
	settings       config.Settings
	machines       map[string]*Machine
	passthrough    map[string]*Passthrough
	gpu            passthrough.Readiness
	servers        map[string]*Server
	consoles       map[string]*Console
	keys           remote.Keys
	metrics        map[model.ID]map[model.Metric]*model.Series
	counters       map[model.ID]counterSample
	jobs           map[string]*runningJob
	events         []model.Event
	notices        []Notice
	catalog        Catalog
	logosRequested map[string]bool
	logoSlots      chan struct{}
	update         update.Info
	loaded         map[Source]bool
	errors         map[Source]string
	starting       map[string]bool
	undo           map[string]undoEntry
	locks          map[model.ID]*sync.Mutex
	log            *events.Log
	nextID         uint64
	rev            uint64
}

func New(o Options) *Store {
	o = normalizeOptions(o)
	s := &Store{
		o:              o,
		ctx:            context.Background(),
		wake:           make(chan struct{}, 1),
		settings:       config.Defaults(),
		machines:       make(map[string]*Machine),
		passthrough:    make(map[string]*Passthrough),
		servers:        make(map[string]*Server),
		consoles:       make(map[string]*Console),
		metrics:        make(map[model.ID]map[model.Metric]*model.Series),
		counters:       make(map[model.ID]counterSample),
		jobs:           make(map[string]*runningJob),
		loaded:         make(map[Source]bool),
		errors:         make(map[Source]string),
		starting:       make(map[string]bool),
		undo:           make(map[string]undoEntry),
		locks:          make(map[model.ID]*sync.Mutex),
		catalog:        Catalog{Logos: make(map[string]string)},
		logosRequested: make(map[string]bool),
		logoSlots:      make(chan struct{}, 12),
	}
	s.publishLocked()
	return s
}

func (s *Store) Run(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.ctx = ctx
	s.mu.Unlock()

	settings, settingsErr := config.Load()
	log, logErr := events.Open(s.o.EventsPath, 500)
	host := Host{
		Engine:      s.o.VM.Caps(s.ctx),
		Setup:       vm.Setup(),
		Passthrough: passthrough.Available(),
		Keyring:     keyring.Available(),
		Consoles:    s.o.Console.Caps(),
		Ryoku:       commandAvailable("ryoku-host"),
		OnBattery:   onBattery(powerSupplyRoot),
	}
	host.Install, _ = update.Detect()

	s.mu.Lock()
	if settingsErr == nil {
		s.settings = settings
	} else {
		s.pushNoticeLocked(model.EventError, "Couldn't load settings: "+firstLine(settingsErr), "", "")
	}
	if logErr == nil {
		s.log = log
		s.events = log.Recent(200)
	} else {
		s.pushNoticeLocked(model.EventError, "Couldn't open the activity log: "+firstLine(logErr), "", "")
	}
	s.host = host
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()

	s.startPollers()
	if settingsErr == nil && settings.Updates.Check && (settings.Updates.LastCheck.IsZero() || s.o.Now().Sub(settings.Updates.LastCheck) >= 24*time.Hour) {
		s.CheckUpdate()
	}
}

func (s *Store) Snapshot() *Snapshot {
	return s.snapshot.Load()
}

func (s *Store) publish() {
	s.mu.Lock()
	if !s.pruneJobsLocked(s.o.Now()) {
		s.mu.Unlock()
		return
	}
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
}

func (s *Store) publishLocked() {
	now := s.o.Now()
	s.pruneJobsLocked(now)
	s.rev++
	snap := &Snapshot{
		Rev:         s.rev,
		At:          now,
		Host:        s.host,
		Settings:    cloneSettings(s.settings),
		Machines:    cloneMachines(s.machines),
		Passthrough: clonePassthrough(s.passthrough),
		GPU:         s.gpu,
		Servers:     cloneServers(s.servers),
		Consoles:    cloneConsoles(s.consoles),
		Keys:        cloneKeys(s.keys),
		Metrics:     s.metrics,
		Jobs:        snapshotJobs(s.jobs),
		Events:      s.events,
		Notices:     s.notices,
		Catalog:     s.catalog,
		Update:      s.update,
		Loaded:      cloneMap(s.loaded),
		Errors:      cloneMap(s.errors),
	}
	snap.Resources = deriveResources(snap, s.starting)
	snap.Alerts = deriveAlerts(snap)
	s.snapshot.Store(snap)
}

func (s *Store) pruneJobsLocked(now time.Time) bool {
	changed := false
	for id, job := range s.jobs {
		if job.job.State != model.JobRunning && !job.job.Finished.IsZero() && now.Sub(job.job.Finished) >= 30*time.Minute {
			delete(s.jobs, id)
			changed = true
		}
	}
	return changed
}

func cloneSettings(in config.Settings) config.Settings {
	out := in
	out.Pinned = append([]string(nil), in.Pinned...)
	out.Defaults.Tools = append([]string(nil), in.Defaults.Tools...)
	return out
}

func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	out := make(map[K]V, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneMachines(in map[string]*Machine) map[string]*Machine {
	return cloneMap(in)
}

func clonePassthrough(in map[string]*Passthrough) map[string]*Passthrough {
	return cloneMap(in)
}

func cloneServers(in map[string]*Server) map[string]*Server {
	return cloneMap(in)
}

func cloneConsoles(in map[string]*Console) map[string]*Console {
	return cloneMap(in)
}

func cloneKeys(in remote.Keys) remote.Keys {
	return remote.Keys{Agent: append([]remote.AgentKey(nil), in.Agent...), Files: append([]remote.FileKey(nil), in.Files...)}
}

func snapshotJobs(in map[string]*runningJob) []model.Job {
	out := make([]model.Job, 0, len(in))
	for _, job := range in {
		out = append(out, job.job)
	}
	sortJobs(out)
	return out
}

func (s *Store) signalPollers() {
	s.wakeMu.Lock()
	close(s.wake)
	s.wake = make(chan struct{})
	s.wakeMu.Unlock()
}

func (s *Store) pollWake() <-chan struct{} {
	s.wakeMu.Lock()
	wake := s.wake
	s.wakeMu.Unlock()
	return wake
}

var _ Ops = (*Store)(nil)
