package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeOps struct {
	name       string
	snap       json.RawMessage
	snapErr    error
	applyErr   error
	restoreErr error
	onApply    func()

	snapCalls    int
	applyCalls   int
	restoreCalls int
	lastOriginal json.RawMessage
	lastPort     int
	lastHTTPS    bool
}

func (f *fakeOps) Name() string { return f.name }

func (f *fakeOps) Snapshot(context.Context) (json.RawMessage, error) {
	f.snapCalls++
	if f.snapErr != nil {
		return nil, f.snapErr
	}
	if f.snap == nil {
		return json.RawMessage(`{"v":"original"}`), nil
	}
	return append(json.RawMessage(nil), f.snap...), nil
}

func (f *fakeOps) Apply(_ context.Context, original json.RawMessage, port int, captureHTTPS bool) error {
	f.applyCalls++
	f.lastOriginal = append(json.RawMessage(nil), original...)
	f.lastPort, f.lastHTTPS = port, captureHTTPS
	if f.onApply != nil {
		f.onApply()
	}
	return f.applyErr
}

func (f *fakeOps) Restore(_ context.Context, original json.RawMessage) error {
	f.restoreCalls++
	f.lastOriginal = append(json.RawMessage(nil), original...)
	return f.restoreErr
}

func newTestManager(t *testing.T, ops platformOps) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	return newWithOps(dir, ops), dir
}

func backupPath(dir string) string { return filepath.Join(dir, backupName) }

func writeTestBackup(t *testing.T, dir, platform, data string) {
	t.Helper()
	raw, err := json.Marshal(backupFile{
		Version:   1,
		Platform:  platform,
		CreatedAt: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC),
		Data:      json.RawMessage(data),
	})
	if err != nil {
		t.Fatalf("marshal backup: %v", err)
	}
	if err := os.WriteFile(backupPath(dir), raw, 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
}

func readBackup(t *testing.T, dir string) backupFile {
	t.Helper()
	raw, err := os.ReadFile(backupPath(dir))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var b backupFile
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("parse backup: %v", err)
	}
	return b
}

func TestEnableSnapshotsOnceAndApplies(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	m, dir := newTestManager(t, f)

	if err := m.Enable(9090, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if f.snapCalls != 1 || f.applyCalls != 1 {
		t.Fatalf("snapshot=%d apply=%d, want 1 and 1", f.snapCalls, f.applyCalls)
	}
	if f.lastPort != 9090 || !f.lastHTTPS {
		t.Fatalf("applied port=%d https=%v, want 9090 and true", f.lastPort, f.lastHTTPS)
	}
	if !m.Active() {
		t.Fatal("the manager is not active after a successful enable")
	}
	b := readBackup(t, dir)
	if b.Platform != "darwin" || string(b.Data) != `{"v":"original"}` {
		t.Fatalf("backup is platform=%q data=%s", b.Platform, b.Data)
	}

	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("second enable: %v", err)
	}
	if f.snapCalls != 1 {
		t.Fatalf("snapshot calls = %d after a second enable, want 1", f.snapCalls)
	}
	if f.applyCalls != 2 || f.lastHTTPS {
		t.Fatalf("second apply calls=%d https=%v, want 2 and false", f.applyCalls, f.lastHTTPS)
	}
	if got := readBackup(t, dir); string(got.Data) != `{"v":"original"}` {
		t.Fatalf("the backup was rewritten to %s", got.Data)
	}
}

func TestBackupIsWrittenBeforeApply(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	m, dir := newTestManager(t, f)
	f.onApply = func() {
		if _, err := os.Stat(backupPath(dir)); err != nil {
			t.Errorf("backup does not exist while apply runs: %v", err)
		}
	}
	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
}

func TestBackupIsPrivate(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	info, err := os.Stat(backupPath(dir))
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("backup mode is %o, want 600", mode)
	}
}

func TestRestorePutsTheOriginalBackAndDeletesTheBackup(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if f.restoreCalls != 1 || string(f.lastOriginal) != `{"v":"original"}` {
		t.Fatalf("restore calls=%d original=%s", f.restoreCalls, f.lastOriginal)
	}
	if _, err := os.Stat(backupPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the backup still exists after restore: %v", err)
	}
	if m.Active() {
		t.Fatal("the manager is still active after restore")
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("second restore: %v", err)
	}
	if f.restoreCalls != 1 {
		t.Fatalf("restore calls = %d after a no-op restore, want 1", f.restoreCalls)
	}
}

func TestRestoreFailureKeepsTheBackup(t *testing.T) {
	f := &fakeOps{name: "darwin", restoreErr: errors.New("restore failed")}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := m.Restore(); err == nil {
		t.Fatal("restore succeeded, want the injected failure")
	}
	if _, err := os.Stat(backupPath(dir)); err != nil {
		t.Fatalf("the backup is gone after a failed restore: %v", err)
	}
	if !m.Active() {
		t.Fatal("the manager must stay active after a failed restore")
	}
}

func TestRecoverRestoresALeftoverBackup(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	dir := t.TempDir()
	writeTestBackup(t, dir, "darwin", `{"v":"saved"}`)

	m := newWithOps(dir, f)
	if err := m.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if f.restoreCalls != 1 || string(f.lastOriginal) != `{"v":"saved"}` {
		t.Fatalf("restore calls=%d original=%s", f.restoreCalls, f.lastOriginal)
	}
	if _, err := os.Stat(backupPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the backup still exists after recover: %v", err)
	}
}

func TestCorruptBackupIsRenamedAndReported(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	dir := t.TempDir()
	if err := os.WriteFile(backupPath(dir), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newWithOps(dir, f)
	if err := m.Recover(); !errors.Is(err, ErrCorruptBackup) {
		t.Fatalf("recover error = %v, want ErrCorruptBackup", err)
	}
	if f.restoreCalls != 0 {
		t.Fatalf("restore ran %d times on a corrupt backup", f.restoreCalls)
	}
	if _, err := os.Stat(backupPath(dir) + ".corrupt"); err != nil {
		t.Fatalf("corrupt backup was not kept beside the new one: %v", err)
	}

	if err := m.Enable(9090, false); !errors.Is(err, ErrCorruptBackup) {
		t.Fatalf("enable error = %v, want ErrCorruptBackup", err)
	}
	if f.snapCalls != 0 || f.applyCalls != 0 {
		t.Fatalf("snapshot=%d apply=%d, want no change after a corrupt backup", f.snapCalls, f.applyCalls)
	}

	m.Reset()
	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("enable after reset: %v", err)
	}
	if f.snapCalls != 1 || f.applyCalls != 1 {
		t.Fatalf("snapshot=%d apply=%d after an explicit reset, want 1 and 1", f.snapCalls, f.applyCalls)
	}
}

func TestExistingBackupIsUsedInsteadOfSnapshotting(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	dir := t.TempDir()
	writeTestBackup(t, dir, "darwin", `{"v":"saved"}`)

	m := newWithOps(dir, f)
	if err := m.Enable(9090, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if f.snapCalls != 0 {
		t.Fatalf("snapshot calls = %d, want 0 with an existing backup", f.snapCalls)
	}
	if string(f.lastOriginal) != `{"v":"saved"}` {
		t.Fatalf("applied original = %s, want the saved backup", f.lastOriginal)
	}
	if got := readBackup(t, dir); string(got.Data) != `{"v":"saved"}` {
		t.Fatalf("the existing backup was rewritten to %s", got.Data)
	}
}

func TestForeignPlatformBackupIsSetAside(t *testing.T) {
	f := &fakeOps{name: "darwin", snap: json.RawMessage(`{"v":"fresh"}`)}
	dir := t.TempDir()
	writeTestBackup(t, dir, "windows", `{"v":"foreign"}`)

	m := newWithOps(dir, f)
	if err := m.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if f.restoreCalls != 0 {
		t.Fatalf("a foreign backup was restored %d times", f.restoreCalls)
	}
	if _, err := os.Stat(backupPath(dir) + ".stale"); err != nil {
		t.Fatalf("foreign backup was not set aside: %v", err)
	}
	if err := m.Enable(9090, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if f.snapCalls != 1 || string(f.lastOriginal) != `{"v":"fresh"}` {
		t.Fatalf("snapshot=%d original=%s, want a fresh snapshot", f.snapCalls, f.lastOriginal)
	}
}

func TestApplyFailureRollsBackAndForgetsTheBackup(t *testing.T) {
	f := &fakeOps{name: "darwin", applyErr: errors.New("apply failed")}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, false); err == nil {
		t.Fatal("enable succeeded, want the injected failure")
	}
	if f.restoreCalls != 1 {
		t.Fatalf("rollback ran %d times, want 1", f.restoreCalls)
	}
	if _, err := os.Stat(backupPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the backup survived a successful rollback: %v", err)
	}
	if m.Active() {
		t.Fatal("the manager is active after a rolled-back apply")
	}
}

func TestApplyAndRollbackFailureKeepsTheBackup(t *testing.T) {
	f := &fakeOps{name: "darwin", applyErr: errors.New("apply failed"), restoreErr: errors.New("restore failed")}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, false); err == nil {
		t.Fatal("enable succeeded, want the injected failure")
	}
	if _, err := os.Stat(backupPath(dir)); err != nil {
		t.Fatalf("the backup is gone after a failed rollback: %v", err)
	}
	if !m.Active() {
		t.Fatal("the manager must stay active when the rollback failed")
	}
}

func TestSnapshotFailureWritesNothing(t *testing.T) {
	f := &fakeOps{name: "darwin", snapErr: errors.New("snapshot failed")}
	m, dir := newTestManager(t, f)
	if err := m.Enable(9090, false); err == nil {
		t.Fatal("enable succeeded, want the injected failure")
	}
	if f.applyCalls != 0 {
		t.Fatalf("apply ran %d times after a failed snapshot", f.applyCalls)
	}
	if _, err := os.Stat(backupPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a backup was written after a failed snapshot: %v", err)
	}
	if m.Active() {
		t.Fatal("the manager is active after a failed snapshot")
	}
}

func TestRestoreWithoutBackupIsNoOp(t *testing.T) {
	f := &fakeOps{name: "darwin"}
	m, _ := newTestManager(t, f)
	if err := m.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if f.restoreCalls != 0 {
		t.Fatalf("restore ran %d times without a backup", f.restoreCalls)
	}
}
