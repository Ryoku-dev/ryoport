package store

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"golang.org/x/sys/unix"
)

type pollSource uint8

const (
	pollMachineList pollSource = iota
	pollMachineStats
	pollSparklines
	pollServerReach
	pollServerProbe
	pollWebApps
	pollPassthrough
)

func interval(source pollSource, focus Focus, selected, battery bool) time.Duration {
	var value time.Duration
	switch source {
	case pollMachineList:
		if focus.Visible {
			value = 10 * time.Second
		} else {
			value = 60 * time.Second
		}
	case pollMachineStats:
		if focus.Visible && focus.Focused {
			value = 2 * time.Second
		} else if focus.Visible {
			value = 5 * time.Second
		}
	case pollSparklines:
		if focus.Visible && focus.Focused {
			value = 5 * time.Second
		} else if focus.Visible {
			value = 10 * time.Second
		}
	case pollServerReach:
		if focus.Visible && focus.Focused {
			value = 15 * time.Second
		} else if focus.Visible {
			value = 30 * time.Second
		}
	case pollServerProbe:
		if focus.Visible && focus.Focused && selected {
			value = 10 * time.Second
		} else if focus.Visible && focus.Focused {
			value = 30 * time.Second
		} else if focus.Visible {
			value = 60 * time.Second
		}
	case pollWebApps:
		if focus.Visible && focus.Focused {
			value = 60 * time.Second
		}
	case pollPassthrough:
		if focus.Visible && focus.Focused {
			value = 10 * time.Second
		} else if focus.Visible {
			value = 30 * time.Second
		}
	}
	if battery && value > 0 {
		value *= 2
	}
	return value
}

func (s *Store) startPollers() {
	go s.cadenceLoop(pollMachineList, s.refreshMachines)
	go s.cadenceLoop(pollServerReach, s.refreshReachability)
	go s.cadenceLoop(pollWebApps, s.refreshApps)
	if s.host.Passthrough {
		go s.cadenceLoop(pollPassthrough, s.refreshPassthrough)
	}
	go s.machineStatsLoop()
	go s.serverProbeLoop()
	go s.batteryLoop()
	go s.watchVMRoot()
	go s.refreshServers()
	go s.refreshConsoles()
	go s.refreshKeys()
}

func (s *Store) cadenceLoop(source pollSource, refresh func()) {
	var last time.Time
	for {
		s.mu.Lock()
		focus, battery, ctx := s.focus, s.host.OnBattery, s.ctx
		s.mu.Unlock()
		cadence := interval(source, focus, false, battery)
		if cadence == 0 {
			select {
			case <-ctx.Done():
				return
			case <-s.pollWake():
				continue
			}
		}
		delay := time.Duration(0)
		if !last.IsZero() {
			delay = cadence - s.o.Now().Sub(last)
			if delay < 0 {
				delay = 0
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.pollWake():
			timer.Stop()
			continue
		case <-timer.C:
			refresh()
			last = s.o.Now()
		}
	}
}

func (s *Store) SetFocus(focus Focus) {
	s.mu.Lock()
	previous := s.focus
	if focus == previous {
		s.mu.Unlock()
		return
	}
	s.focus = focus
	s.mu.Unlock()
	s.signalPollers()
	if focus.Selected != previous.Selected && focus.Selected.Kind() == model.KindMachine {
		go s.refreshMachineAux(focus.Selected.Key())
	}
}

func (s *Store) Refresh() {
	go s.refreshMachines()
	go s.refreshServers()
	go s.refreshConsoles()
	go s.refreshKeys()
	go s.refreshReachability()
	go s.refreshAllProbes()
	go s.refreshApps()
	s.mu.Lock()
	selected := s.focus.Selected
	s.mu.Unlock()
	if selected.Kind() == model.KindMachine {
		go s.refreshMachineAux(selected.Key())
	}
	if s.Snapshot().Host.Passthrough {
		go s.refreshPassthrough()
	}
}

func (s *Store) refreshMachines() {
	ctx := s.currentContext()
	list, err := s.o.VM.List(ctx)
	if err != nil {
		s.setSourceError(SourceMachines, err)
		return
	}
	result := make(map[string]*Machine, len(list))
	slugs := make([]string, 0, len(list))
	for _, item := range list {
		detail, detailErr := s.o.VM.Get(ctx, item.Name)
		if detailErr != nil {
			detail = vm.Detail{Machine: item}
		}
		s.mu.Lock()
		old := s.machines[item.Name]
		logo := s.catalog.Logos[detail.OS]
		s.mu.Unlock()
		machine := &Machine{Detail: detail, Logo: logo}
		if old != nil {
			machine.Stats, machine.USB, machine.Forwards = old.Stats, old.USB, old.Forwards
			if old.Logo != "" && old.OS == detail.OS {
				machine.Logo = old.Logo
			}
		}
		if detail.OS != "" {
			slugs = append(slugs, detail.OS)
		}
		result[item.Name] = machine
	}
	changed := s.applyMachines(result)
	s.resolveLogos(slugs)
	if changed {
		s.signalPollers()
	}
}

func (s *Store) applyMachines(result map[string]*Machine) bool {
	s.mu.Lock()
	changed := !reflect.DeepEqual(s.machines, result)
	if changed {
		s.machines = result
	}
	for name, machine := range result {
		if machine.Running {
			if _, ok := s.starting[name]; ok {
				delete(s.starting, name)
				changed = true
			}
		}
	}
	changed = s.markSourceSuccessLocked(SourceMachines) || changed
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
	return changed
}

func (s *Store) refreshServers() {
	ctx := s.currentContext()
	hosts, err := s.o.Remote.List(ctx)
	if err != nil {
		s.setSourceError(SourceServers, err)
		return
	}
	result := make(map[string]*Server, len(hosts))
	s.mu.Lock()
	for _, host := range hosts {
		server := &Server{Host: host}
		if old := s.servers[host.Alias]; old != nil {
			server.Reach, server.Probe = old.Reach, old.Probe
			server.Apps, server.Guests, server.Tunnels = old.Apps, old.Guests, old.Tunnels
		}
		result[host.Alias] = server
	}
	changed := !reflect.DeepEqual(s.servers, result)
	if changed {
		s.servers = result
	}
	changed = s.markSourceSuccessLocked(SourceServers) || changed
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
		s.signalPollers()
	}
}

func (s *Store) refreshConsoles() {
	profiles, err := s.o.Console.List()
	if err != nil {
		s.setSourceError(SourceConsoles, err)
		return
	}
	result := make(map[string]*Console, len(profiles))
	s.mu.Lock()
	for _, profile := range profiles {
		entry := &Console{Profile: profile}
		if old := s.consoles[profile.ID]; old != nil {
			entry.Reach = old.Reach
		}
		result[profile.ID] = entry
	}
	changed := !reflect.DeepEqual(s.consoles, result)
	if changed {
		s.consoles = result
	}
	changed = s.markSourceSuccessLocked(SourceConsoles) || changed
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) refreshKeys() {
	keys, err := s.o.Remote.Keys(s.currentContext())
	if err != nil {
		s.setSourceError(SourceKeys, err)
		return
	}
	s.mu.Lock()
	changed := !reflect.DeepEqual(s.keys, keys)
	if changed {
		s.keys = keys
	}
	changed = s.markSourceSuccessLocked(SourceKeys) || changed
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) refreshReachability() {
	ctx := s.currentContext()
	reaches := s.o.Remote.PingAll(ctx)
	consoleReaches := s.o.Console.ProbeAll(ctx)
	s.mu.Lock()
	changed := false
	for _, reach := range reaches {
		if server := s.servers[reach.Alias]; server != nil && (server.Reach == nil || *server.Reach != reach) {
			copyValue := *server
			value := reach
			copyValue.Reach = &value
			s.servers[reach.Alias] = &copyValue
			changed = true
		}
	}
	for _, reach := range consoleReaches {
		if profile := s.consoles[reach.ID]; profile != nil && (profile.Reach == nil || *profile.Reach != reach) {
			copyValue := *profile
			value := reach
			copyValue.Reach = &value
			s.consoles[reach.ID] = &copyValue
			changed = true
		}
	}
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) refreshApps() {
	checks := s.o.Remote.AppCheckAll(s.currentContext())
	grouped := make(map[string][]remote.AppCheck)
	for _, check := range checks {
		grouped[check.Alias] = append(grouped[check.Alias], check)
	}
	s.mu.Lock()
	changed := false
	for alias, server := range s.servers {
		values := grouped[alias]
		if !reflect.DeepEqual(server.Apps, values) {
			copyValue := *server
			copyValue.Apps = append([]remote.AppCheck(nil), values...)
			s.servers[alias] = &copyValue
			changed = true
		}
	}
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) refreshPassthrough() {
	ctx := s.currentContext()
	list, err := passthrough.Status(ctx)
	if err != nil {
		s.setSourceError(SourcePassthrough, err)
		return
	}
	result := make(map[string]*Passthrough, len(list.Machines))
	sampled := make(map[string]bool, len(list.Machines))
	for _, item := range list.Machines {
		entry := &Passthrough{Machine: item}
		if passthroughState(item.State) == model.StateRunning {
			s.mu.Lock()
			if old := s.passthrough[item.Name]; old != nil {
				entry.Stats = old.Stats
			}
			s.mu.Unlock()
			if stats, statsErr := passthrough.ReadStats(ctx, item.Name); statsErr == nil {
				entry.Stats = &stats
				sampled[item.Name] = stats.Running
			}
		}
		result[item.Name] = entry
	}
	gpu := passthrough.Readiness{Ready: list.Ready, Blocker: list.Blocker}
	s.mu.Lock()
	changed := !reflect.DeepEqual(s.passthrough, result) || s.gpu != gpu
	if changed {
		s.passthrough = result
		s.gpu = gpu
	}
	metricsChanged := false
	for name := range sampled {
		metricsChanged = s.pushPassthroughMetricsLocked(name, result[name]) || metricsChanged
	}
	changed = s.markSourceSuccessLocked(SourcePassthrough) || changed || metricsChanged
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) machineStatsLoop() {
	last := make(map[string]time.Time)
	var selected model.ID
	for {
		s.mu.Lock()
		focus, battery, ctx := s.focus, s.host.OnBattery, s.ctx
		machines := make(map[string]*Machine, len(s.machines))
		for name, machine := range s.machines {
			machines[name] = machine
		}
		s.mu.Unlock()
		if focus.Selected != selected {
			selected = focus.Selected
			if selected.Kind() == model.KindMachine {
				delete(last, selected.Key())
			}
		}
		now := s.o.Now()
		var next time.Duration
		var due []string
		for name, machine := range machines {
			if !machine.Running {
				continue
			}
			isSelected := selected == model.NewID(model.KindMachine, name)
			source := pollSparklines
			if isSelected {
				source = pollMachineStats
			}
			cadence := interval(source, focus, isSelected, battery)
			if cadence == 0 {
				continue
			}
			remaining := cadence - now.Sub(last[name])
			if last[name].IsZero() || remaining <= 0 {
				due = append(due, name)
				continue
			}
			if next == 0 || remaining < next {
				next = remaining
			}
		}
		if len(due) > 0 {
			for _, name := range due {
				s.refreshMachineStats(name)
				last[name] = s.o.Now()
			}
			continue
		}
		if next == 0 {
			select {
			case <-ctx.Done():
				return
			case <-s.pollWake():
				continue
			}
		}
		timer := time.NewTimer(next)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.pollWake():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *Store) refreshMachineStats(name string) {
	stats, err := s.o.VM.Stats(s.currentContext(), name)
	if err != nil {
		return
	}
	s.mu.Lock()
	machine := s.machines[name]
	if machine == nil {
		s.mu.Unlock()
		return
	}
	statsChanged := !reflect.DeepEqual(machine.Stats, &stats)
	if statsChanged {
		copyValue := *machine
		copyValue.Stats = &stats
		machine = &copyValue
		s.machines[name] = machine
	}
	metricsChanged := false
	if machine.Running && stats.Running {
		metricsChanged = s.pushMachineMetricsLocked(name, machine)
	}
	changed := statsChanged || metricsChanged
	if changed {
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) refreshMachineAux(name string) {
	ctx := s.currentContext()
	usb, usbErr := s.o.VM.USB(ctx, name)
	forwards, forwardErr := s.o.VM.PortForwards(name)
	if usbErr != nil && forwardErr != nil {
		return
	}
	s.mu.Lock()
	machine := s.machines[name]
	changed := false
	if machine != nil {
		copyValue := *machine
		if usbErr == nil && !reflect.DeepEqual(copyValue.USB, usb) {
			copyValue.USB = usb
			changed = true
		}
		if forwardErr == nil && !reflect.DeepEqual(copyValue.Forwards, forwards) {
			copyValue.Forwards = forwards
			changed = true
		}
		if changed {
			s.machines[name] = &copyValue
			s.publishLocked()
		}
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) serverProbeLoop() {
	last := make(map[string]time.Time)
	var selected model.ID
	for {
		s.mu.Lock()
		focus, battery, ctx := s.focus, s.host.OnBattery, s.ctx
		aliases := make([]string, 0, len(s.servers))
		for alias := range s.servers {
			aliases = append(aliases, alias)
		}
		s.mu.Unlock()
		if focus.Selected != selected {
			selected = focus.Selected
			if selected.Kind() == model.KindServer {
				delete(last, selected.Key())
			}
		}
		now := s.o.Now()
		var next time.Duration
		var due []string
		for _, alias := range aliases {
			isSelected := selected == model.NewID(model.KindServer, alias)
			cadence := interval(pollServerProbe, focus, isSelected, battery)
			if cadence == 0 {
				continue
			}
			remaining := cadence - now.Sub(last[alias])
			if last[alias].IsZero() || remaining <= 0 {
				due = append(due, alias)
				continue
			}
			if next == 0 || remaining < next {
				next = remaining
			}
		}
		if len(due) > 0 {
			for _, alias := range due {
				s.refreshProbe(alias)
				last[alias] = s.o.Now()
			}
			continue
		}
		if next == 0 {
			select {
			case <-ctx.Done():
				return
			case <-s.pollWake():
				continue
			}
		}
		timer := time.NewTimer(next)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.pollWake():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *Store) refreshAllProbes() {
	s.mu.Lock()
	aliases := make([]string, 0, len(s.servers))
	for alias := range s.servers {
		aliases = append(aliases, alias)
	}
	s.mu.Unlock()
	for _, alias := range aliases {
		s.refreshProbe(alias)
	}
}

func (s *Store) refreshProbe(alias string) {
	ctx := s.currentContext()
	probe := s.o.Remote.Probe(ctx, alias)
	var guests []remote.Guest
	s.mu.Lock()
	server := s.servers[alias]
	s.mu.Unlock()
	if server != nil && server.Pve != nil {
		guests, _ = s.o.Remote.PVEGuests(ctx, alias)
	}
	tunnels, _ := s.o.Remote.Tunnels(alias)
	s.mu.Lock()
	server = s.servers[alias]
	changed := false
	if server != nil {
		copyValue := *server
		if !reflect.DeepEqual(server.Probe, &probe) {
			copyValue.Probe = &probe
			changed = true
		}
		if !reflect.DeepEqual(server.Guests, guests) {
			copyValue.Guests = append([]remote.Guest(nil), guests...)
			changed = true
		}
		if !reflect.DeepEqual(server.Tunnels, tunnels) {
			copyValue.Tunnels = append([]remote.Tunnel(nil), tunnels...)
			changed = true
		}
		if changed {
			server = &copyValue
			s.servers[alias] = server
		}
		metricsChanged := s.pushServerMetricsLocked(alias, server)
		changed = changed || metricsChanged
		if changed {
			s.publishLocked()
		}
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
	if server != nil {
		if slug := build.DistroSlug(probe.Distro); slug != "" {
			s.resolveLogos([]string{slug})
		}
	}
}

func (s *Store) pushMachineMetricsLocked(name string, machine *Machine) bool {
	stats := machine.Stats
	if stats == nil {
		return false
	}
	id := model.NewID(model.KindMachine, name)
	now := s.o.Now()
	previous := s.counters[id]
	samples := make([]metricSample, 0, 4)
	if stats.HostCPUPct != nil {
		value := *stats.HostCPUPct / 100
		if stats.VCPUs != nil && *stats.VCPUs > 0 {
			value /= float64(*stats.VCPUs)
		}
		samples = append(samples, metricSample{metric: model.MetricCPU, value: clamp01(value)})
	}
	if stats.HostRSSMB != nil {
		if total := ramMB(machine.RAM); total > 0 {
			samples = append(samples, metricSample{metric: model.MetricMem, value: clamp01(float64(*stats.HostRSSMB) / float64(total))})
		}
	}
	disk, net := qmpCounters(stats)
	if !previous.at.IsZero() {
		samples = append(samples,
			metricSample{metric: model.MetricDisk, value: rate(now, disk, previous, previous.disk)},
			metricSample{metric: model.MetricNet, value: rate(now, net, previous, previous.net)},
		)
	}
	s.counters[id] = counterSample{at: now, disk: disk, net: net}
	return s.pushMetricsLocked(id, samples, now)
}

func qmpCounters(stats *qmp.Stats) (uint64, uint64) {
	var disk, net uint64
	if stats.DiskReadBytes != nil {
		disk += *stats.DiskReadBytes
	}
	if stats.DiskWriteBytes != nil {
		disk += *stats.DiskWriteBytes
	}
	if stats.NetRXBytes != nil {
		net += *stats.NetRXBytes
	}
	if stats.NetTXBytes != nil {
		net += *stats.NetTXBytes
	}
	return disk, net
}

func (s *Store) pushServerMetricsLocked(alias string, server *Server) bool {
	probe := server.Probe
	if probe == nil || !probe.OK {
		return false
	}
	id := model.NewID(model.KindServer, alias)
	now := s.o.Now()
	samples := []metricSample{{metric: model.MetricCPU, value: clamp01(probe.CPUPercent / 100)}}
	if probe.MemTotalKb > 0 {
		samples = append(samples, metricSample{metric: model.MetricMem, value: clamp01(1 - float64(probe.MemAvailKb)/float64(probe.MemTotalKb))})
	}
	disk := uint64(max(0, probe.DiskReadBytes) + max(0, probe.DiskWriteBytes))
	net := uint64(max(0, probe.NetRxBytes) + max(0, probe.NetTxBytes))
	previous := s.counters[id]
	if !previous.at.IsZero() {
		samples = append(samples,
			metricSample{metric: model.MetricDisk, value: rate(now, disk, previous, previous.disk)},
			metricSample{metric: model.MetricNet, value: rate(now, net, previous, previous.net)},
		)
	}
	s.counters[id] = counterSample{at: now, disk: disk, net: net}
	return s.pushMetricsLocked(id, samples, now)
}

func (s *Store) pushPassthroughMetricsLocked(name string, machine *Passthrough) bool {
	stats := machine.Stats
	if stats == nil || !stats.Running {
		return false
	}
	id := model.NewID(model.KindPassthrough, name)
	now := s.o.Now()
	previous := s.counters[id]
	samples := make([]metricSample, 0, 4)
	if !previous.at.IsZero() {
		seconds := now.Sub(previous.at).Seconds()
		if seconds > 0 && stats.CPUTimeNS >= previous.cpuTime && machine.VCPUs > 0 {
			cpu := float64(stats.CPUTimeNS-previous.cpuTime) / (seconds * float64(time.Second) * float64(machine.VCPUs))
			samples = append(samples, metricSample{metric: model.MetricCPU, value: clamp01(cpu)})
		}
	}
	if machine.RAMMB > 0 {
		samples = append(samples, metricSample{metric: model.MetricMem, value: clamp01(float64(stats.MemoryKB) / float64(machine.RAMMB*1024))})
	}
	disk := stats.DiskReadBytes + stats.DiskWriteBytes
	net := stats.NetReceiveBytes + stats.NetSendBytes
	if !previous.at.IsZero() {
		samples = append(samples,
			metricSample{metric: model.MetricDisk, value: rate(now, disk, previous, previous.disk)},
			metricSample{metric: model.MetricNet, value: rate(now, net, previous, previous.net)},
		)
	}
	s.counters[id] = counterSample{at: now, disk: disk, net: net, cpuTime: stats.CPUTimeNS}
	return s.pushMetricsLocked(id, samples, now)
}

type metricSample struct {
	metric model.Metric
	value  float64
}

func (s *Store) pushMetricsLocked(id model.ID, samples []metricSample, now time.Time) bool {
	if len(samples) == 0 {
		return false
	}
	metrics := cloneMap(s.metrics[id])
	for _, sample := range samples {
		series := metrics[sample.metric]
		if series == nil {
			series = model.NewSeries(150)
		} else {
			series = series.Clone()
		}
		series.Push(sample.value, now)
		metrics[sample.metric] = series
	}
	all := cloneMap(s.metrics)
	all[id] = metrics
	s.metrics = all
	return true
}

func ramMB(value string) int {
	text := strings.TrimSpace(strings.ToUpper(value))
	multiplier := 1
	if strings.HasSuffix(text, "G") {
		multiplier = 1024
		text = strings.TrimSuffix(text, "G")
	} else {
		text = strings.TrimSuffix(text, "M")
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0
	}
	return n * multiplier
}

func (s *Store) batteryLoop() {
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.currentContext().Done():
			return
		case <-timer.C:
			on := onBattery(powerSupplyRoot)
			s.mu.Lock()
			changed := on != s.host.OnBattery
			if changed {
				s.host.OnBattery = on
				s.publishLocked()
			}
			s.mu.Unlock()
			if changed {
				s.o.Invalidate()
				s.signalPollers()
			}
			timer.Reset(60 * time.Second)
		}
	}
}

func (s *Store) watchVMRoot() {
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return
	}
	defer unix.Close(fd)
	pipe := make([]int, 2)
	if err := unix.Pipe2(pipe, unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return
	}
	defer unix.Close(pipe[0])
	defer unix.Close(pipe[1])
	ctx := s.currentContext()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_, _ = unix.Write(pipe[1], []byte{1})
		case <-stop:
		}
	}()
	watch := -1
	var debounce *time.Timer
	buffer := make([]byte, unix.SizeofInotifyEvent*64)
	for {
		if watch < 0 {
			if _, err := os.Stat(s.o.VM.Root); err == nil {
				watch, _ = unix.InotifyAddWatch(fd, s.o.VM.Root, unix.IN_CREATE|unix.IN_DELETE|unix.IN_CLOSE_WRITE|unix.IN_MOVED_FROM|unix.IN_MOVED_TO|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF)
			}
		}
		s.mu.Lock()
		cadence := interval(pollMachineList, s.focus, false, s.host.OnBattery)
		s.mu.Unlock()
		pollFD := []unix.PollFd{
			{Fd: int32(fd), Events: unix.POLLIN},
			{Fd: int32(pipe[0]), Events: unix.POLLIN},
		}
		_, _ = unix.Poll(pollFD, int(cadence/time.Millisecond))
		if pollFD[1].Revents&unix.POLLIN != 0 {
			if debounce != nil {
				debounce.Stop()
			}
			return
		}
		select {
		case <-s.currentContext().Done():
			if debounce != nil {
				debounce.Stop()
			}
			return
		default:
		}
		if pollFD[0].Revents&unix.POLLIN == 0 {
			continue
		}
		n, readErr := unix.Read(fd, buffer)
		if readErr != nil && !errors.Is(readErr, unix.EAGAIN) {
			return
		}
		if n == 0 {
			continue
		}
		for offset := 0; offset+unix.SizeofInotifyEvent <= n; {
			mask := binary.NativeEndian.Uint32(buffer[offset+4:])
			nameLength := int(binary.NativeEndian.Uint32(buffer[offset+12:]))
			if mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF|unix.IN_IGNORED) != 0 {
				watch = -1
			}
			offset += unix.SizeofInotifyEvent + nameLength
		}
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(300*time.Millisecond, func() {
			if s.currentContext().Err() == nil {
				s.refreshMachines()
			}
		})
	}
}

func (s *Store) setSourceError(source Source, err error) {
	s.mu.Lock()
	text := firstLine(err)
	changed := !s.loaded[source] || s.errors[source] != text
	if changed {
		s.loaded[source] = true
		s.errors[source] = text
		s.publishLocked()
	}
	s.mu.Unlock()
	if changed {
		s.o.Invalidate()
	}
}

func (s *Store) markSourceSuccessLocked(source Source) bool {
	changed := !s.loaded[source]
	if _, ok := s.errors[source]; ok {
		changed = true
	}
	s.loaded[source] = true
	delete(s.errors, source)
	return changed
}

func (s *Store) currentContext() context.Context {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	return ctx
}
