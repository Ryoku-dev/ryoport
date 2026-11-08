package store

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/launch"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/passthrough"
	"github.com/ryoku-dev/ryoport/internal/qmp"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/vm"
)

func (s *Store) resourceLock(id model.ID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[id] = lock
	}
	return lock
}

func (s *Store) runAction(target model.ID, success, failure string, fn func(context.Context) error, after func(bool)) {
	go func() {
		lock := s.resourceLock(target)
		lock.Lock()
		defer lock.Unlock()
		err := fn(s.currentContext())
		if after != nil {
			after(err == nil)
		}
		if err != nil {
			s.finishAction(model.EventError, "Couldn't "+failure, target, err)
			return
		}
		s.finishAction(model.EventOK, success, target, nil)
	}()
}

func (s *Store) finishAction(level model.EventLevel, text string, target model.ID, err error) {
	if err != nil {
		text += ": " + firstLine(err)
	}
	s.mu.Lock()
	s.recordLocked(level, text, target, firstLine(err))
	s.pushNoticeLocked(level, text, target, "")
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
}

func (s *Store) recordLocked(level model.EventLevel, text string, target model.ID, detail string) {
	event := model.Event{At: s.o.Now(), Target: target, Level: level, Text: text, Detail: detail}
	s.events = append([]model.Event{event}, s.events...)
	if len(s.events) > 200 {
		s.events = s.events[:200]
	}
	if s.log != nil {
		_ = s.log.Append(event)
	}
}

func (s *Store) pushNoticeLocked(level model.EventLevel, text string, target model.ID, undo string) {
	s.nextID++
	notice := Notice{ID: s.nextID, Level: level, Text: text, Target: target, Undo: undo}
	next := append(append([]Notice(nil), s.notices...), notice)
	if len(next) > 3 {
		next = append([]Notice(nil), next[len(next)-3:]...)
	}
	s.notices = next
}

func (s *Store) Primary(id model.ID) {
	snap := s.Snapshot()
	resource, ok := snap.Resource(id)
	if !ok {
		return
	}
	switch resource.Kind {
	case model.KindMachine:
		machine := snap.Machines[id.Key()]
		if resource.Primary == model.VerbStart {
			display := vm.Display(snap.Settings.Defaults.Display)
			if display != vm.DisplaySpice && display != vm.DisplayHeadless {
				display = vm.DisplayWindow
			}
			s.Start(id.Key(), display, false)
		} else if resource.Primary == model.VerbOpen && machine != nil && (machine.Display == string(vm.DisplayHeadless) || machine.Display == "none") && machine.SSH != "" {
			s.OpenSSH(id.Key())
		} else if resource.Primary == model.VerbOpen {
			s.OpenViewer(id.Key())
		}
	case model.KindPassthrough:
		if resource.Primary == model.VerbStart || resource.Primary == model.VerbOpen {
			s.StartPassthrough(id.Key())
		}
	case model.KindServer:
		if resource.Primary == model.VerbSetPass {
			s.mu.Lock()
			s.pushNoticeLocked(model.EventWarn, "Enter the password for "+id.Key(), id, "")
			s.publishLocked()
			s.mu.Unlock()
			s.o.Invalidate()
		} else {
			s.Connect(id.Key())
		}
	case model.KindConsole:
		s.OpenConsole(id.Key())
	}
}

func pinnedOwner(id model.ID) string {
	if id.Kind() == model.KindServer {
		return "remote"
	}
	return "settings"
}

func (s *Store) SetPinned(id model.ID, pinned bool) {
	if pinnedOwner(id) == "remote" {
		s.runAction(id, pinnedMessage(id.Key(), pinned), "change pin for "+id.Key(), func(context.Context) error {
			return s.o.Remote.SetPinned(id.Key(), pinned)
		}, func(ok bool) {
			if ok {
				go s.refreshServers()
			}
		})
		return
	}
	s.UpdateSettings(func(settings *config.Settings) {
		value := string(id)
		filtered := settings.Pinned[:0]
		for _, current := range settings.Pinned {
			if current != value {
				filtered = append(filtered, current)
			}
		}
		settings.Pinned = filtered
		if pinned {
			settings.Pinned = append(settings.Pinned, value)
		}
	})
}

func pinnedMessage(name string, pinned bool) string {
	if pinned {
		return "Pinned " + name
	}
	return "Unpinned " + name
}

func (s *Store) UpdateSettings(change func(*config.Settings)) {
	if change == nil {
		return
	}
	go func() {
		lock := s.resourceLock("settings:")
		lock.Lock()
		defer lock.Unlock()
		s.mu.Lock()
		settings := cloneSettings(s.settings)
		s.mu.Unlock()
		change(&settings)
		if err := config.Save(settings); err != nil {
			s.finishAction(model.EventError, "Couldn't save settings", "", err)
			return
		}
		s.mu.Lock()
		s.settings = settings
		s.recordLocked(model.EventOK, "Saved settings", "", "")
		s.pushNoticeLocked(model.EventOK, "Saved settings", "", "")
		s.publishLocked()
		s.mu.Unlock()
		s.o.Invalidate()
		s.signalPollers()
	}()
}

func (s *Store) Start(name string, display vm.Display, disposable bool) {
	id := model.NewID(model.KindMachine, name)
	s.mu.Lock()
	s.starting[name] = true
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
	s.runAction(id, "Started "+name, "start "+name, func(ctx context.Context) error {
		return s.o.VM.Launch(ctx, name, display, disposable)
	}, func(ok bool) {
		if !ok {
			s.mu.Lock()
			delete(s.starting, name)
			s.publishLocked()
			s.mu.Unlock()
			s.o.Invalidate()
		}
		go s.refreshMachines()
	})
}

func (s *Store) Stop(name string, force bool) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Stopped "+name, "stop "+name, func(ctx context.Context) error { return s.o.VM.Stop(ctx, name, force) }, func(bool) { go s.refreshMachines() })
}

func (s *Store) OpenViewer(name string) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Opened "+name, "open "+name, func(context.Context) error { return s.o.VM.OpenViewer(name) }, nil)
}

func (s *Store) OpenSSH(name string) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Opened SSH to "+name, "open SSH to "+name, func(context.Context) error {
		argv, err := s.o.VM.SSHCommand(name)
		if err != nil {
			return err
		}
		return launch.Terminal(argv, "dev.ryoku.ryoport_ssh", name)
	}, nil)
}

func (s *Store) Power(name string, action qmp.PowerAction) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Sent "+string(action)+" to "+name, "send "+string(action)+" to "+name, func(ctx context.Context) error { return s.o.VM.Power(ctx, name, action) }, nil)
}

func (s *Store) Balloon(name string, mb int) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Changed memory for "+name, "change memory for "+name, func(ctx context.Context) error { return s.o.VM.Balloon(ctx, name, mb) }, nil)
}

func (s *Store) PinCPU(name string, on bool) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Changed CPU pinning for "+name, "change CPU pinning for "+name, func(ctx context.Context) error { return s.o.VM.Pin(ctx, name, on) }, nil)
}

func (s *Store) SetConfig(name, key, value string) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Saved "+name, "save "+name, func(context.Context) error { return s.o.VM.SetConfig(name, key, value) }, func(bool) { go s.refreshMachines() })
}

func (s *Store) CreateSnapshot(name, tag string) {
	s.machineAction(name, "Created snapshot "+tag+" for "+name, "create snapshot for "+name, func(ctx context.Context) error { return s.o.VM.CreateSnapshot(ctx, name, tag) })
}

func (s *Store) RestoreSnapshot(name, tag string) {
	s.machineAction(name, "Restored snapshot "+tag+" for "+name, "restore snapshot for "+name, func(ctx context.Context) error { return s.o.VM.RestoreSnapshot(ctx, name, tag) })
}

func (s *Store) DeleteSnapshot(name, tag string) {
	s.machineAction(name, "Deleted snapshot "+tag+" from "+name, "delete snapshot from "+name, func(ctx context.Context) error { return s.o.VM.DeleteSnapshot(ctx, name, tag) })
}

func (s *Store) Rename(old, new string) {
	s.machineAction(old, "Renamed "+old+" to "+new, "rename "+old, func(context.Context) error { return s.o.VM.Rename(old, new) })
}

func (s *Store) Resize(name, size string) {
	s.machineAction(name, "Resized "+name, "resize "+name, func(ctx context.Context) error { return s.o.VM.Resize(ctx, name, size) })
}

func (s *Store) Seal(name string) {
	s.machineAction(name, "Sealed "+name, "seal "+name, func(ctx context.Context) error { return s.o.VM.Seal(ctx, name) })
}

func (s *Store) RestoreSeal(name string) {
	s.machineAction(name, "Restored "+name, "restore "+name, func(ctx context.Context) error { return s.o.VM.RestoreSeal(ctx, name) })
}

func (s *Store) MakeTemplate(name, template string) {
	s.machineAction(name, "Created template "+template, "create template "+template, func(ctx context.Context) error { return s.o.Build.MakeTemplate(ctx, name, template) })
}

func (s *Store) Delete(name string, diskOnly bool) {
	s.machineAction(name, "Deleted "+name, "delete "+name, func(ctx context.Context) error { return s.o.VM.Delete(ctx, name, diskOnly) })
}

func (s *Store) OpenFolder(name string) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Opened folder for "+name, "open folder for "+name, func(context.Context) error { return launch.OpenPath(s.o.VM.Folder(name)) }, nil)
}

func (s *Store) SetUSB(name, idValue string, on bool) {
	s.machineAuxAction(name, "Changed USB devices for "+name, "change USB devices for "+name, func(context.Context) error { return s.o.VM.SetUSB(name, idValue, on) })
}

func (s *Store) AddForward(name, spec string) {
	s.machineAuxAction(name, "Added port forward to "+name, "add port forward to "+name, func(context.Context) error { return s.o.VM.AddPortForward(name, spec) })
}

func (s *Store) RemoveForward(name, spec string) {
	s.machineAuxAction(name, "Removed port forward from "+name, "remove port forward from "+name, func(context.Context) error { return s.o.VM.RemovePortForward(name, spec) })
}

func (s *Store) machineAuxAction(name, success, failure string, fn func(context.Context) error) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, success, failure, fn, func(bool) { go s.refreshMachineAux(name) })
}

func (s *Store) machineAction(name, success, failure string, fn func(context.Context) error) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, success, failure, fn, func(bool) { go s.refreshMachines() })
}

func (s *Store) VirtIO(name string) string {
	target := model.ID("")
	if name != "" {
		target = model.NewID(model.KindMachine, name)
	}
	return s.startJob("Download VirtIO driver CD", target, "Downloaded", func(ctx context.Context, emit func(build.Event)) (string, error) {
		_, err := s.o.Build.VirtIO(ctx, name, emit)
		return "VirtIO driver CD", err
	})
}

func (s *Store) Create(osName, release, edition string) string {
	return s.startJob("Create "+osName, "", "Created", func(ctx context.Context, emit func(build.Event)) (string, error) {
		return s.o.Build.Create(ctx, osName, release, edition, emit)
	})
}

func (s *Store) Import(name, iso, guest, osName string) {
	id := model.NewID(model.KindMachine, name)
	s.runAction(id, "Imported "+name, "import "+name, func(context.Context) error { return s.o.Build.Import(name, iso, guest, osName) }, func(bool) { go s.refreshMachines() })
}

func (s *Store) Instant(options build.InstantOptions) string {
	name := options.Name
	if name == "" {
		name = options.OS + "-instant"
	}
	target := model.NewID(model.KindMachine, name)
	return s.startJob("Create "+name, target, "Created", func(ctx context.Context, emit func(build.Event)) (string, error) {
		return s.o.Build.Instant(ctx, options, emit)
	})
}

func (s *Store) Spawn(template, name string, disposable bool) string {
	target := model.NewID(model.KindMachine, name)
	return s.startJob("Create "+name, target, "Created", func(ctx context.Context, _ func(build.Event)) (string, error) {
		return s.o.Build.Spawn(ctx, template, name, disposable)
	})
}

func (s *Store) InstallEngine() {
	advice := s.Snapshot().Host.Setup
	if len(advice.Argv) == 0 {
		text := advice.Hint
		if text == "" {
			text = "Install quickemu with your system package manager."
		}
		s.mu.Lock()
		s.pushNoticeLocked(model.EventWarn, text, "", "")
		s.publishLocked()
		s.mu.Unlock()
		s.o.Invalidate()
		return
	}
	s.runAction("", "Opened the VM engine installer", "open the VM engine installer", func(context.Context) error {
		return launch.Terminal(advice.Argv, "dev.ryoku.ryoport_install", "Install VM engine")
	}, nil)
}

func (s *Store) StartPassthrough(name string) {
	id := model.NewID(model.KindPassthrough, name)
	s.runAction(id, "Started "+name, "start "+name, func(ctx context.Context) error { return passthrough.Start(ctx, name) }, func(bool) { go s.refreshPassthrough() })
}

func (s *Store) StopPassthrough(name string) {
	id := model.NewID(model.KindPassthrough, name)
	s.runAction(id, "Stopped "+name, "stop "+name, func(ctx context.Context) error { return passthrough.Stop(ctx, name) }, func(bool) { go s.refreshPassthrough() })
}

func (s *Store) RemovePassthrough(name string, deleteDisk bool) {
	id := model.NewID(model.KindPassthrough, name)
	s.runAction(id, "Removed "+name, "remove "+name, func(ctx context.Context) error { return passthrough.Remove(ctx, name, deleteDisk) }, func(bool) { go s.refreshPassthrough() })
}

func (s *Store) CreatePassthrough(options passthrough.CreateOptions) string {
	target := model.NewID(model.KindPassthrough, options.Name)
	return s.startJob("Create "+options.Name, target, "Created", func(ctx context.Context, _ func(build.Event)) (string, error) {
		return options.Name, passthrough.Create(ctx, options)
	})
}

func (s *Store) OpenHubGPU() {
	s.runAction("", "Opened GPU settings", "open GPU settings", func(context.Context) error { return passthrough.OpenHubGPU() }, nil)
}

func (s *Store) Connect(alias string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Connected to "+alias, "connect to "+alias, func(context.Context) error { return s.o.Remote.Connect(alias) }, nil)
}

func (s *Store) OpenFiles(alias string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Opened files on "+alias, "open files on "+alias, func(ctx context.Context) error {
		return s.o.Remote.OpenFiles(ctx, alias)
	}, nil)
}

func (s *Store) RunCommand(alias, command string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Opened command on "+alias, "run command on "+alias, func(context.Context) error { return s.o.Remote.RunInTerminal(alias, command) }, nil)
}

func (s *Store) SaveServer(input remote.HostInput, previous string) {
	id := model.NewID(model.KindServer, input.Alias)
	s.runAction(id, "Saved "+input.Alias, "save "+input.Alias, func(context.Context) error {
		if err := s.o.Remote.Save(input); err != nil {
			return err
		}
		if previous != "" && previous != input.Alias {
			return s.o.Remote.Remove(previous)
		}
		return nil
	}, func(bool) { go s.refreshServers() })
}

func (s *Store) RemoveServer(alias string) {
	id := model.NewID(model.KindServer, alias)
	s.mu.Lock()
	server := s.servers[alias]
	if server == nil {
		s.mu.Unlock()
		return
	}
	input := hostInput(server.Host)
	token := s.newUndoTokenLocked()
	done := make(chan struct{})
	s.undo[token] = undoEntry{expires: s.o.Now().Add(10 * time.Second), host: &input, done: done}
	delete(s.servers, alias)
	s.pushNoticeLocked(model.EventWarn, "Removed "+alias, id, token)
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
	s.runAction(id, "Removed "+alias, "remove "+alias, func(context.Context) error { return s.o.Remote.Remove(alias) }, func(ok bool) {
		close(done)
		if !ok {
			go s.refreshServers()
		}
	})
}

func (s *Store) SetPassword(alias, password string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Saved password for "+alias, "save password for "+alias, func(context.Context) error { return s.o.Remote.SetPassword(alias, password) }, func(bool) { go s.refreshAllProbes() })
}

func (s *Store) ClearPassword(alias string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Cleared password for "+alias, "clear password for "+alias, func(context.Context) error { return s.o.Remote.ClearPassword(alias) }, nil)
}

func (s *Store) DeployKey(alias, publicKey string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Deployed key to "+alias, "deploy key to "+alias, func(context.Context) error { return s.o.Remote.CopyID(alias, publicKey) }, func(bool) { go s.refreshAllProbes() })
}

func (s *Store) Keygen(name, kind string) {
	s.runAction("", "Created SSH key "+name, "create SSH key "+name, func(context.Context) error {
		_, err := s.o.Remote.Keygen(name, kind)
		return err
	}, func(bool) { go s.refreshKeys() })
}

func (s *Store) ForgetHostKey(alias string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Forgot host key for "+alias, "forget host key for "+alias, func(context.Context) error { return s.o.Remote.KnownRemove(alias) }, nil)
}

func (s *Store) OpenTunnel(alias, spec string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Opened tunnel to "+alias, "open tunnel to "+alias, func(context.Context) error {
		_, err := s.o.Remote.OpenTunnel(alias, spec)
		return err
	}, func(bool) { go s.refreshProbe(alias) })
}

func (s *Store) CloseTunnel(idValue string) {
	s.runAction("", "Closed tunnel", "close tunnel", func(context.Context) error { return s.o.Remote.CloseTunnel(idValue) }, nil)
}

func (s *Store) PVEAction(alias, node, kind string, vmid int, action string) {
	id := model.NewID(model.KindServer, alias)
	s.runAction(id, "Sent "+action+" to guest", "send "+action+" to guest", func(ctx context.Context) error { return s.o.Remote.PVEAction(ctx, alias, node, kind, vmid, action) }, func(bool) { go s.refreshProbe(alias) })
}

func (s *Store) OpenURL(url string) {
	s.runAction("", "Opened link", "open link", func(context.Context) error { return launch.OpenURL(url) }, nil)
}

func (s *Store) OpenConsole(idValue string) {
	id := model.NewID(model.KindConsole, idValue)
	s.runAction(id, "Opened console", "open console", func(context.Context) error { return s.o.Console.Open(idValue) }, nil)
}

func (s *Store) SaveConsole(profile console.Profile) {
	id := model.NewID(model.KindConsole, profile.ID)
	s.runAction(id, "Saved "+profile.Name, "save "+profile.Name, func(context.Context) error { return s.o.Console.Save(profile) }, func(bool) { go s.refreshConsoles() })
}

func (s *Store) RemoveConsole(idValue string) {
	id := model.NewID(model.KindConsole, idValue)
	s.mu.Lock()
	profile := s.consoles[idValue]
	if profile == nil {
		s.mu.Unlock()
		return
	}
	copyProfile := profile.Profile
	token := s.newUndoTokenLocked()
	done := make(chan struct{})
	s.undo[token] = undoEntry{expires: s.o.Now().Add(10 * time.Second), console: &copyProfile, done: done}
	delete(s.consoles, idValue)
	s.pushNoticeLocked(model.EventWarn, "Removed "+profile.Name, id, token)
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
	s.runAction(id, "Removed "+profile.Name, "remove "+profile.Name, func(context.Context) error { return s.o.Console.Remove(idValue) }, func(ok bool) {
		close(done)
		if !ok {
			go s.refreshConsoles()
		}
	})
}

func (s *Store) SetConsolePassword(idValue, password string) {
	id := model.NewID(model.KindConsole, idValue)
	s.runAction(id, "Saved console password", "save console password", func(context.Context) error { return s.o.Console.SetPassword(idValue, password) }, nil)
}

func (s *Store) ClearConsolePassword(idValue string) {
	id := model.NewID(model.KindConsole, idValue)
	s.runAction(id, "Cleared console password", "clear console password", func(context.Context) error { return s.o.Console.ClearPassword(idValue) }, nil)
}

func (s *Store) newUndoTokenLocked() string {
	s.nextID++
	return "undo-" + strconv.FormatUint(s.nextID, 10)
}

func (s *Store) Undo(token string) {
	s.mu.Lock()
	entry, ok := takeUndo(s.undo, token, s.o.Now())
	s.mu.Unlock()
	if !ok {
		return
	}
	if entry.host != nil {
		input := *entry.host
		go func() {
			if waitUndoRemoval(s.currentContext(), entry.done) != nil {
				return
			}
			s.runAction(model.NewID(model.KindServer, input.Alias), "Restored "+input.Alias, "restore "+input.Alias, func(context.Context) error {
				return s.o.Remote.Save(input)
			}, func(bool) { go s.refreshServers() })
		}()
	}
	if entry.console != nil {
		profile := *entry.console
		go func() {
			if waitUndoRemoval(s.currentContext(), entry.done) != nil {
				return
			}
			s.runAction(model.NewID(model.KindConsole, profile.ID), "Restored "+profile.Name, "restore "+profile.Name, func(context.Context) error {
				return s.o.Console.Save(profile)
			}, func(bool) { go s.refreshConsoles() })
		}()
	}
}

func waitUndoRemoval(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func takeUndo(entries map[string]undoEntry, token string, now time.Time) (undoEntry, bool) {
	entry, ok := entries[token]
	if ok {
		delete(entries, token)
	}
	if !ok || !now.Before(entry.expires) {
		return undoEntry{}, false
	}
	return entry, true
}

func hostInput(host remote.Host) remote.HostInput {
	return remote.HostInput{
		Alias: host.Alias, HostName: host.HostName, User: host.User, Port: host.Port,
		IdentityFile: host.IdentityFile, ProxyJump: host.ProxyJump, Group: host.Group,
		Tags: append([]string(nil), host.Tags...), Notes: host.Notes, Pinned: host.Pinned,
		Watch: append([]string(nil), host.Watch...), Apps: append([]remote.App(nil), host.Apps...),
		Pve: host.Pve, Auth: host.Auth,
	}
}
