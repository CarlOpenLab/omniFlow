// Package store keeps OmniFlow's collections, history and preferences in
// one JSON file under the app's user-data directory.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

// Store is the app's saved data, loaded once and written when it changes.
type Store struct {
	mu   sync.Mutex
	path string
	data model.Data
}

// Open loads the file at path, seeding a collection on a fresh install.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &s.data); err != nil {
			// A corrupt file must not take the app down: keep it beside the
			// new one so it can be looked at, and start fresh.
			_ = os.Rename(path, path+".corrupt")
			s.data = seed()
		}
	case errors.Is(err, os.ErrNotExist):
		s.data = seed()
	default:
		return nil, err
	}
	if s.data.Prefs.ProxyPort == "" {
		s.data.Prefs = model.DefaultPrefs()
	}
	if s.data.Requests == nil {
		s.data.Requests = []model.Request{}
	}
	for i := range s.data.Requests {
		s.data.Requests[i].Normalize()
	}
	if s.data.History == nil {
		s.data.History = []model.HistoryEntry{}
	}
	if s.data.Environments == nil {
		s.data.Environments = []model.Environment{}
	}
	return s, nil
}

func seed() model.Data {
	return model.Data{
		Prefs:    model.DefaultPrefs(),
		Requests: model.SeedRequests(),
		History:  []model.HistoryEntry{},
	}
}

// Path is the file the store writes.
func (s *Store) Path() string { return s.path }

// Data returns a snapshot of the saved data. The app builds its window from
// its own state and calls Sync to write changes, so the copy is not hot.
func (s *Store) Data() model.Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.Data{
		Prefs:        s.data.Prefs,
		Requests:     append([]model.Request(nil), s.data.Requests...),
		History:      append([]model.HistoryEntry(nil), s.data.History...),
		Environments: append([]model.Environment(nil), s.data.Environments...),
	}
}

// Prefs returns the saved preferences.
func (s *Store) Prefs() model.Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Prefs
}

// SetPrefs replaces the preferences and writes the file.
func (s *Store) SetPrefs(p model.Prefs) error {
	s.mu.Lock()
	s.data.Prefs = p
	s.mu.Unlock()
	return s.write()
}

// SetRequests replaces the saved collection and writes the file.
func (s *Store) SetRequests(reqs []model.Request) error {
	s.mu.Lock()
	s.data.Requests = append([]model.Request(nil), reqs...)
	s.mu.Unlock()
	return s.write()
}

// SetHistory replaces the recent list and writes the file.
func (s *Store) SetHistory(entries []model.HistoryEntry) error {
	s.mu.Lock()
	s.data.History = append([]model.HistoryEntry(nil), entries...)
	s.mu.Unlock()
	return s.write()
}

// SetEnvironments replaces the environments and writes the file.
func (s *Store) SetEnvironments(envs []model.Environment) error {
	s.mu.Lock()
	s.data.Environments = make([]model.Environment, len(envs))
	for i, e := range envs {
		s.data.Environments[i] = e.Clone()
	}
	s.mu.Unlock()
	return s.write()
}

// write saves all of the data atomically: a crash mid-write leaves the old
// file rather than half a file.
func (s *Store) write() error {
	s.mu.Lock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".omniflow-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
