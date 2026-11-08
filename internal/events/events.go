// Package events stores the bounded activity log.
package events

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ryoku-dev/ryoport/internal/model"
)

type Log struct {
	mu             sync.RWMutex
	path           string
	keep           int
	lineCount      int
	needsSeparator bool
	events         []model.Event
}

func Open(path string, keep int) (*Log, error) {
	if keep < 1 {
		return nil, errors.New("event retention must be positive")
	}
	if path == "" || !filepath.IsAbs(path) {
		return nil, errors.New("event log path must be absolute")
	}
	log := &Log{path: path, keep: keep, events: make([]model.Event, 0, keep)}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return log, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		log.lineCount++
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		log.events = append(log.events, event)
		if len(log.events) > keep {
			copy(log.events, log.events[len(log.events)-keep:])
			log.events = log.events[:keep]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 0 {
		var last [1]byte
		if _, err := file.ReadAt(last[:], info.Size()-1); err != nil {
			return nil, err
		}
		log.needsSeparator = last[0] != '\n'
	}
	return log, nil
}

func (log *Log) Append(event model.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	log.mu.Lock()
	defer log.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(log.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(log.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if log.needsSeparator {
		if _, err := file.Write([]byte{'\n'}); err != nil {
			file.Close()
			return err
		}
		log.needsSeparator = false
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	log.lineCount++
	log.events = append(log.events, event)
	if len(log.events) > log.keep {
		copy(log.events, log.events[len(log.events)-log.keep:])
		log.events = log.events[:log.keep]
	}
	if log.lineCount > 2*log.keep {
		if err := log.rewriteLocked(); err != nil {
			return err
		}
	}
	return nil
}

func (log *Log) Recent(n int) []model.Event {
	log.mu.RLock()
	defer log.mu.RUnlock()
	return newest(log.events, n, "")
}

func (log *Log) For(id model.ID, n int) []model.Event {
	log.mu.RLock()
	defer log.mu.RUnlock()
	return newest(log.events, n, id)
}

func newest(events []model.Event, n int, id model.ID) []model.Event {
	if n <= 0 {
		return []model.Event{}
	}
	result := make([]model.Event, 0, min(n, len(events)))
	for i := len(events) - 1; i >= 0 && len(result) < n; i-- {
		if id == "" || events[i].Target == id {
			result = append(result, events[i])
		}
	}
	return result
}

func (log *Log) rewriteLocked() error {
	dir := filepath.Dir(log.path)
	tmp, err := os.CreateTemp(dir, ".events-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	encoder := json.NewEncoder(tmp)
	for _, event := range log.events {
		if err := encoder.Encode(event); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, log.path); err != nil {
		return err
	}
	log.needsSeparator = false
	log.lineCount = len(log.events)
	return nil
}
