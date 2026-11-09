// Package sysproxy points the machine's system proxy at a local capture
// proxy and puts the previous settings back afterwards.
//
// The proxy settings are changed only around a capture session: Enable
// snapshots what the machine had, writes it beside the app's data first, and
// Restore puts it back. A snapshot left by a crash is restored on the next
// start. The snapshot is the only record of the original settings, so it is
// never overwritten while it exists.
package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// backupName is the file holding the settings from before OmniFlow changed
// them, beside the app's own data.
const backupName = "system-proxy-backup.json"

// ErrCorruptBackup means the snapshot could not be read. The machine's proxy
// settings are left untouched and the file is kept as .corrupt.
var ErrCorruptBackup = errors.New("the system proxy backup is corrupt")

const operationTimeout = 30 * time.Second

// platformOps is what a platform does to the system proxy. Snapshot reads
// the current settings; Apply points them at the local proxy; Restore puts
// the snapshot back.
type platformOps interface {
	// Name is the platform the snapshot came from, so a backup written on
	// another system is not applied here.
	Name() string
	Snapshot(ctx context.Context) (json.RawMessage, error)
	Apply(ctx context.Context, original json.RawMessage, port int, captureHTTPS bool) error
	Restore(ctx context.Context, original json.RawMessage) error
}

// backupFile is the on-disk form of a snapshot.
type backupFile struct {
	Version   int             `json:"version"`
	Platform  string          `json:"platform"`
	CreatedAt time.Time       `json:"createdAt"`
	Data      json.RawMessage `json:"data"`
}

// Manager owns the machine's system proxy for one capture session.
type Manager struct {
	dir string
	ops platformOps

	mu       sync.Mutex
	active   bool
	original json.RawMessage
	corrupt  bool
}

// New returns a Manager that keeps its backup in dir, using the platform's
// own proxy settings.
func New(dir string) *Manager {
	return newWithOps(dir, defaultOps())
}

func newWithOps(dir string, ops platformOps) *Manager {
	return &Manager{dir: dir, ops: ops}
}

// Active reports whether the manager has changed the system proxy and has not
// restored it yet.
func (m *Manager) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// Enable points the system proxy at 127.0.0.1:port. The first call snapshots
// the previous settings; later calls reuse that snapshot, so a second call
// can never mistake OmniFlow's own proxy for the machine's original.
func (m *Manager) Enable(port int, captureHTTPS bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	if m.corrupt {
		return ErrCorruptBackup
	}
	if m.original == nil {
		saved, err := m.readBackup()
		if err != nil {
			return err
		}
		if saved == nil {
			snap, err := m.ops.Snapshot(ctx)
			if err != nil {
				return err
			}
			if err := m.writeBackup(snap); err != nil {
				return err
			}
			saved = snap
		}
		m.original = saved
	}

	if err := m.ops.Apply(ctx, m.original, port, captureHTTPS); err != nil {
		// A half-applied proxy is worse than the failure itself: put the
		// snapshot back now, and keep it if that also fails so the next
		// stop or start can try again.
		if restoreErr := m.ops.Restore(ctx, m.original); restoreErr != nil {
			m.active = true
			return fmt.Errorf("%w (putting the previous proxy back also failed: %v)", err, restoreErr)
		}
		_ = m.removeBackup()
		m.original = nil
		m.active = false
		return err
	}
	m.active = true
	return nil
}

// Restore puts the machine's previous proxy settings back and removes the
// backup. With no backup it does nothing.
func (m *Manager) Restore() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	original := m.original
	if original == nil {
		saved, err := m.readBackup()
		if err != nil {
			return err
		}
		if saved == nil {
			return nil
		}
		original = saved
	}
	if err := m.ops.Restore(ctx, original); err != nil {
		return err
	}
	if err := m.removeBackup(); err != nil {
		return err
	}
	m.original = nil
	m.active = false
	return nil
}

// Recover restores a snapshot left behind by a crash or a force-quit. It runs
// before the proxy starts, so no capture can be pointing at settings that are
// about to change.
func (m *Manager) Recover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	saved, err := m.readBackup()
	if err != nil {
		return err
	}
	if saved == nil {
		return nil
	}
	if err := m.ops.Restore(ctx, saved); err != nil {
		return err
	}
	return m.removeBackup()
}

// Reset forgets a corrupt backup for this session. The app calls it only when
// the person explicitly turns automatic proxy control back on, which is the
// one moment a fresh snapshot of the current settings is wanted.
func (m *Manager) Reset() {
	m.mu.Lock()
	m.corrupt = false
	m.mu.Unlock()
}

// readBackup loads the saved settings. A missing file is not an error. A
// backup from another platform is set aside, and a corrupt one is renamed and
// reported so the caller can leave the system alone.
func (m *Manager) readBackup() (json.RawMessage, error) {
	path := filepath.Join(m.dir, backupName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var b backupFile
	if err := json.Unmarshal(raw, &b); err != nil || b.Version != 1 || len(b.Data) == 0 {
		_ = os.Rename(path, path+".corrupt")
		m.corrupt = true
		return nil, ErrCorruptBackup
	}
	if b.Platform != m.ops.Name() {
		_ = os.Rename(path, path+".stale")
		return nil, nil
	}
	return b.Data, nil
}

func (m *Manager) writeBackup(data json.RawMessage) error {
	b := backupFile{
		Version:   1,
		Platform:  m.ops.Name(),
		CreatedAt: time.Now().UTC(),
		Data:      data,
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	// Write beside the real file, then rename it into place: a crash leaves
	// either the old backup or the complete new one, never half of one.
	tmp, err := os.CreateTemp(m.dir, ".system-proxy-*.tmp")
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
	return os.Rename(tmpName, filepath.Join(m.dir, backupName))
}

func (m *Manager) removeBackup() error {
	err := os.Remove(filepath.Join(m.dir, backupName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// unsupportedOps is the platform ops for a system this package does not know
// how to change.
type unsupportedOps struct{ name string }

func (u unsupportedOps) Name() string {
	if u.name == "" {
		return "unsupported"
	}
	return u.name
}

func (unsupportedOps) Snapshot(context.Context) (json.RawMessage, error) {
	return nil, errors.New("changing the system proxy is not supported on this platform")
}

func (unsupportedOps) Apply(context.Context, json.RawMessage, int, bool) error {
	return errors.New("changing the system proxy is not supported on this platform")
}

func (unsupportedOps) Restore(context.Context, json.RawMessage) error {
	return errors.New("changing the system proxy is not supported on this platform")
}
