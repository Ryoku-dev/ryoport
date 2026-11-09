// Package store holds the state the window draws: one immutable snapshot,
// the pollers that refresh it, and the actions that change things.
package store

import (
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

type Page string

const (
	PageFleet    Page = "fleet"
	PageActivity Page = "activity"
	PageSettings Page = "settings"
	PageCreate   Page = "create"
)

// Focus is what the window shows. Pollers pick their cadence from it, so an
// unwatched window costs nothing.
type Focus struct {
	Visible  bool
	Focused  bool
	Page     Page
	Selected model.ID
	Tab      string
}

// Source names one thing the store loads, for Loaded and Errors.
type Source string

const (
	SourceMachines    Source = "machines"
	SourcePassthrough Source = "passthrough"
	SourceServers     Source = "servers"
	SourceConsoles    Source = "consoles"
	SourceKeys        Source = "keys"
	SourceCatalog     Source = "catalog"
	SourceUpdate      Source = "update"
)

// Host is what this computer can do. It decides which kinds and actions the
// window offers at all.
type Host struct {
	Engine      vm.Caps
	Setup       vm.SetupAdvice
	Passthrough bool
	Keyring     bool
	Consoles    console.Caps
	Ryoku       bool
	OnBattery   bool
	Install     update.Install
}

// HostStats is the local computer sample shown by the fleet board.
type HostStats struct {
	CPUFraction float64
	CPUValid    bool
	Cores       int
	MemoryUsed  uint64
	MemoryTotal uint64
	VMDiskFree  uint64
	VMDiskTotal uint64
	SampledAt   time.Time
}

type Machine struct {
	vm.Detail
	Stats    *qmp.Stats
	USB      []vm.USBDevice
	Forwards []vm.PortForward
	Logo     string
}

type Passthrough struct {
	passthrough.Machine
	Stats *passthrough.Stats
}

type Server struct {
	remote.Host
	Reach   *remote.Reach
	Probe   *remote.Probe
	Apps    []remote.AppCheck
	Guests  []remote.Guest
	Tunnels []remote.Tunnel
}

type Console struct {
	console.Profile
	Reach *console.Reach
}

type Catalog struct {
	OS        []build.OS
	Cloud     []build.CloudImage
	Templates []build.Template
	ToolSets  []build.ToolSet
	Logos     map[string]string
}

// Notice is one line of feedback for the toast stack. Undo, when set, is the
// token Ops.Undo takes back.
type Notice struct {
	ID     uint64
	Level  model.EventLevel
	Text   string
	Target model.ID
	Undo   string
}

// Snapshot is never mutated after it is published; a frame reads it without
// locks. Every map, slice and pointed-to value is immutable and may be shared
// with later snapshots. UI code must never modify any of them. Maps are keyed
// by the resource key (machine name, alias, console id).
type Snapshot struct {
	Rev         uint64
	At          time.Time
	Host        Host
	HostStats   HostStats
	Settings    config.Settings
	Resources   []model.Resource
	Machines    map[string]*Machine
	Passthrough map[string]*Passthrough
	GPU         passthrough.Readiness
	Servers     map[string]*Server
	Consoles    map[string]*Console
	Keys        remote.Keys
	Metrics     map[model.ID]map[model.Metric]*model.Series
	Jobs        []model.Job
	Alerts      []model.Alert
	Events      []model.Event
	Notices     []Notice
	Catalog     Catalog
	Update      update.Info
	Loaded      map[Source]bool
	Errors      map[Source]string
}

func (s *Snapshot) Resource(id model.ID) (model.Resource, bool) {
	for _, r := range s.Resources {
		if r.ID == id {
			return r, true
		}
	}
	return model.Resource{}, false
}

func (s *Snapshot) Series(id model.ID, m model.Metric) *model.Series {
	if s.Metrics == nil {
		return nil
	}
	return s.Metrics[id][m]
}

// JobFor returns the running job that targets id, if any.
func (s *Snapshot) JobFor(id model.ID) (model.Job, bool) {
	for _, j := range s.Jobs {
		if j.Target == id && j.State == model.JobRunning {
			return j, true
		}
	}
	return model.Job{}, false
}

func (s *Snapshot) AlertsFor(id model.ID) []model.Alert {
	var out []model.Alert
	for _, a := range s.Alerts {
		if a.Target == id {
			out = append(out, a)
		}
	}
	return out
}

// Ops is every action the window can take. Every method returns at once; the
// work runs on a worker, and its result shows up in the next snapshot as
// state, a job, an event and a notice. Methods that start a job return its id.
type Ops interface {
	SetFocus(Focus)
	Refresh()
	Primary(id model.ID)
	SetPinned(id model.ID, pinned bool)
	Undo(token string)
	Cancel(jobID string)

	Start(name string, display vm.Display, disposable bool)
	Stop(name string, force bool)
	OpenViewer(name string)
	OpenSSH(name string)
	Power(name string, action qmp.PowerAction)
	Balloon(name string, mb int)
	PinCPU(name string, on bool)
	SetConfig(name, key, value string)
	CreateSnapshot(name, tag string)
	RestoreSnapshot(name, tag string)
	DeleteSnapshot(name, tag string)
	Rename(old, new string)
	Resize(name, size string)
	Seal(name string)
	RestoreSeal(name string)
	MakeTemplate(name, template string)
	Delete(name string, diskOnly bool)
	OpenFolder(name string)
	SetUSB(name, id string, on bool)
	AddForward(name, spec string)
	RemoveForward(name, spec string)
	VirtIO(name string) string

	LoadCatalog(refresh bool)
	Create(osName, release, edition string) string
	Import(name, iso, guest, osName string)
	Instant(options build.InstantOptions) string
	Spawn(template, name string, disposable bool) string
	InstallEngine()

	StartPassthrough(name string)
	StopPassthrough(name string)
	RemovePassthrough(name string, deleteDisk bool)
	CreatePassthrough(options passthrough.CreateOptions) string
	OpenHubGPU()

	Connect(alias string)
	OpenFiles(alias string)
	RunCommand(alias, command string)
	SaveServer(input remote.HostInput, previous string)
	RemoveServer(alias string)
	SetPassword(alias, password string)
	ClearPassword(alias string)
	DeployKey(alias, publicKey string)
	Keygen(name, kind string)
	ForgetHostKey(alias string)
	OpenTunnel(alias, spec string)
	CloseTunnel(id string)
	PVEAction(alias, node, kind string, vmid int, action string)
	OpenURL(url string)

	OpenConsole(id string)
	SaveConsole(profile console.Profile)
	RemoveConsole(id string)
	SetConsolePassword(id, password string)
	ClearConsolePassword(id string)

	UpdateSettings(change func(*config.Settings))
	CheckUpdate()
	ApplyUpdate()
	Relaunch()
}
