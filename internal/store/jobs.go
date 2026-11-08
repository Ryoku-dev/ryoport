package store

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/config"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

type jobRunner func(context.Context, func(build.Event)) (string, error)

func (s *Store) startJob(title string, target model.ID, successVerb string, run jobRunner) string {
	s.mu.Lock()
	s.nextID++
	id := "job-" + strconv.FormatUint(s.nextID, 10)
	ctx, cancel := context.WithCancel(s.ctx)
	s.jobs[id] = &runningJob{job: model.Job{
		ID: id, Target: target, Title: title, Phase: "Starting", Progress: -1,
		Started: s.o.Now(), State: model.JobRunning, Cancelable: true,
	}, cancel: cancel}
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()

	go func() {
		if target != "" {
			lock := s.resourceLock(target)
			lock.Lock()
			defer lock.Unlock()
		}
		result, err := run(ctx, func(event build.Event) { s.applyJobEvent(id, event) })
		s.finishJob(id, result, successVerb, err)
	}()
	return id
}

func (s *Store) applyJobEvent(id string, event build.Event) {
	s.mu.Lock()
	running := s.jobs[id]
	if running == nil || running.job.State != model.JobRunning {
		s.mu.Unlock()
		return
	}
	applyBuildEvent(&running.job, event)
	if running.job.Target == "" && event.Name != "" && event.Name != "virtio-win" {
		running.job.Target = model.NewID(model.KindMachine, event.Name)
	}
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
}

func applyBuildEvent(job *model.Job, event build.Event) {
	switch event.Phase {
	case build.PhaseResolve:
		job.Phase = "Resolving"
	case build.PhaseDownload:
		job.Phase = "Downloading"
	case build.PhaseConfig:
		job.Phase = "Writing config"
	case build.PhaseDone:
		job.Phase = "Finishing"
	case build.PhaseCancelled:
		job.Phase = "Cancelling"
	case build.PhaseError:
		job.Phase = firstNonempty(event.Message, "Failed")
	case build.PhaseLog:
		if line := cleanLogLine(event.Line); line != "" {
			job.Detail = line
		}
	}
	if event.Progress != nil {
		job.Done = event.Progress.Recv
		job.Total = event.Progress.Total
		job.Rate = event.Progress.BPS
		job.Progress = -1
		if job.Total > 0 {
			job.Progress = clamp01(float64(job.Done) / float64(job.Total))
		}
	}
}

// cleanLogLine turns a line of quickget or download output into something a
// person reads at a glance: list bullets go, a bare "URL: ..." line becomes
// the host it comes from, and runs of spaces collapse.
func cleanLogLine(line string) string {
	line = strings.Join(strings.Fields(line), " ")
	for _, bullet := range []string{"- ", "* ", "> "} {
		line = strings.TrimPrefix(line, bullet)
	}
	if rest, ok := strings.CutPrefix(line, "URL: "); ok {
		if parsed, err := url.Parse(strings.TrimSpace(rest)); err == nil && parsed.Host != "" {
			return "From " + parsed.Host
		}
	}
	const most = 120
	if runes := []rune(line); len(runes) > most {
		line = string(runes[:most-1]) + "…"
	}
	return line
}

func (s *Store) finishJob(id, result, successVerb string, err error) {
	s.mu.Lock()
	running := s.jobs[id]
	if running == nil {
		s.mu.Unlock()
		return
	}
	running.job.Finished = s.o.Now()
	running.job.Cancelable = false
	level := model.EventOK
	text := running.job.Title + " finished"
	if successVerb != "" {
		name := result
		if name == "" {
			name = running.job.Target.Key()
		}
		if name != "" {
			text = successVerb + " " + name
		}
	}
	if errors.Is(err, context.Canceled) {
		running.job.State = model.JobCancelled
		running.job.Phase = "Cancelled"
		level = model.EventWarn
		text = "Cancelled " + strings.ToLower(running.job.Title)
	} else if err != nil {
		running.job.State = model.JobFailed
		running.job.Err = firstLine(err)
		running.job.Phase = running.job.Err
		level = model.EventError
		text = "Couldn't " + strings.ToLower(running.job.Title) + ": " + running.job.Err
	} else {
		running.job.State = model.JobDone
		running.job.Phase = "Done"
		running.job.Progress = 1
	}
	target := running.job.Target
	s.recordLocked(level, text, target, firstLine(err))
	s.pushNoticeLocked(level, text, target, "")
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
	time.AfterFunc(30*time.Minute, s.publish)
	if err == nil {
		switch target.Kind() {
		case model.KindMachine:
			go s.refreshMachines()
		case model.KindPassthrough:
			go s.refreshPassthrough()
		}
	}
}

func (s *Store) Cancel(jobID string) {
	s.mu.Lock()
	job := s.jobs[jobID]
	if job != nil && job.job.State == model.JobRunning && job.cancel != nil {
		job.job.Phase = "Cancelling"
		job.cancel()
		s.publishLocked()
	}
	s.mu.Unlock()
	if job != nil {
		s.o.Invalidate()
	}
}

func sortJobs(jobs []model.Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if (jobs[i].State == model.JobRunning) != (jobs[j].State == model.JobRunning) {
			return jobs[i].State == model.JobRunning
		}
		return jobs[i].Started.After(jobs[j].Started)
	})
}

func (s *Store) LoadCatalog(refresh bool) {
	go func() {
		catalog, err := s.o.Build.Catalog(s.currentContext(), refresh)
		if err != nil {
			s.setSourceError(SourceCatalog, err)
			return
		}
		templates, templateErr := build.Templates()
		if templateErr != nil {
			s.setSourceError(SourceCatalog, templateErr)
			return
		}
		s.mu.Lock()
		s.catalog.OS = catalog
		s.catalog.Cloud = build.Cloud()
		s.catalog.Templates = templates
		s.catalog.ToolSets = build.ToolSets()
		if s.catalog.Logos == nil {
			s.catalog.Logos = make(map[string]string)
		}
		s.loaded[SourceCatalog] = true
		delete(s.errors, SourceCatalog)
		s.publishLocked()
		s.mu.Unlock()
		s.o.Invalidate()
		go s.prefetchLogos(catalog)
	}()
}

func (s *Store) prefetchLogos(systems []build.OS) {
	slugs := make([]string, 0, len(systems))
	for _, system := range systems {
		slugs = append(slugs, system.OS)
	}
	s.resolveLogos(slugs)
}

func (s *Store) resolveLogos(slugs []string) {
	seen := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		if seen[slug] || !validLogoSlug(slug) {
			continue
		}
		seen[slug] = true
		s.mu.Lock()
		resolved := s.catalog.Logos[slug] != ""
		requested := s.logosRequested[slug]
		s.mu.Unlock()
		if resolved {
			continue
		}
		if path := cachedLogo(slug); path != "" {
			s.publishLogo(slug, path)
			continue
		}
		if requested {
			continue
		}
		s.mu.Lock()
		if s.catalog.Logos[slug] != "" || s.logosRequested[slug] {
			s.mu.Unlock()
			continue
		}
		s.logosRequested[slug] = true
		s.mu.Unlock()
		go s.fetchLogo(slug)
	}
}

func (s *Store) fetchLogo(slug string) {
	ctx := s.currentContext()
	select {
	case s.logoSlots <- struct{}{}:
	case <-ctx.Done():
		return
	}
	logo := s.o.Build.Prefetch(ctx, []string{slug})[slug]
	<-s.logoSlots
	if logo != "" {
		s.publishLogo(slug, logo)
	}
}

func cachedLogo(slug string) string {
	for _, extension := range []string{".svg", ".png"} {
		path := filepath.Join(xdg.IconCache(), slug+extension)
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return path
		}
	}
	return ""
}

func validLogoSlug(slug string) bool {
	if slug == "" {
		return false
	}
	first := slug[0]
	if (first < 'a' || first > 'z') && (first < '0' || first > '9') {
		return false
	}
	for _, char := range slug {
		alpha := char >= 'a' && char <= 'z'
		digit := char >= '0' && char <= '9'
		if alpha || digit || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Store) publishLogo(osName, logo string) {
	s.mu.Lock()
	updated := cloneMap(s.catalog.Logos)
	updated[osName] = logo
	s.catalog.Logos = updated
	for name, machine := range s.machines {
		if machine.OS == osName {
			copyValue := *machine
			copyValue.Logo = logo
			s.machines[name] = &copyValue
		}
	}
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
}

func (s *Store) CheckUpdate() {
	go func() {
		info, err := s.o.Update.Check(s.currentContext())
		s.mu.Lock()
		s.loaded[SourceUpdate] = true
		s.settings.Updates.LastCheck = s.o.Now()
		if err != nil {
			s.errors[SourceUpdate] = firstLine(err)
		} else {
			s.update = info
			delete(s.errors, SourceUpdate)
		}
		if saveErr := config.Save(s.settings); saveErr != nil && err == nil {
			s.errors[SourceUpdate] = firstLine(saveErr)
		}
		s.publishLocked()
		s.mu.Unlock()
		s.o.Invalidate()
	}()
}

func (s *Store) ApplyUpdate() {
	s.mu.Lock()
	info := s.update
	s.mu.Unlock()
	s.startJob("Apply update", "", "Updated", func(ctx context.Context, _ func(build.Event)) (string, error) {
		err := s.o.Update.Apply(ctx, info, func(done, total int64) {
			s.applyUpdateProgress(done, total)
		})
		if err == nil {
			s.mu.Lock()
			s.update.Available = false
			s.update.RestartPending = true
			s.publishLocked()
			s.mu.Unlock()
			s.o.Invalidate()
		}
		return "Ryoport", err
	})
}

func (s *Store) applyUpdateProgress(done, total int64) {
	s.mu.Lock()
	for _, running := range s.jobs {
		if running.job.State == model.JobRunning && running.job.Title == "Apply update" {
			running.job.Phase = "Downloading"
			running.job.Done = done
			running.job.Total = total
			running.job.Progress = -1
			if total > 0 {
				running.job.Progress = clamp01(float64(done) / float64(total))
			}
			break
		}
	}
	s.publishLocked()
	s.mu.Unlock()
	s.o.Invalidate()
}

func (s *Store) Relaunch() {
	go func() {
		if err := update.Relaunch(); err != nil {
			s.finishAction(model.EventError, "Couldn't relaunch Ryoport", "", err)
			return
		}
		s.o.Quit()
	}()
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
