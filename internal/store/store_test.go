package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

func TestFreshStoreSeedsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "omniflow.json")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := len(s.Data().Requests); got == 0 {
		t.Fatal("a fresh store has no seeded requests")
	}
	if got := s.Prefs().ProxyPort; got != "9090" {
		t.Errorf("default proxy port %q", got)
	}

	prefs := model.DefaultPrefs()
	prefs.ProxyPort = "9099"
	prefs.FollowFlows = false
	requests := []model.Request{{ID: "req_1", Name: "Saved", Method: "GET", URL: "https://example.com"}}
	history := []model.HistoryEntry{{ID: "h_1", Method: "GET", URL: "https://example.com", Status: 200}}
	if err := s.SetPrefs(prefs); err != nil {
		t.Fatalf("SetPrefs: %v", err)
	}
	if err := s.SetRequests(requests); err != nil {
		t.Fatalf("SetRequests: %v", err)
	}
	if err := s.SetHistory(history); err != nil {
		t.Fatalf("SetHistory: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	data := reopened.Data()
	if data.Prefs.ProxyPort != "9099" || data.Prefs.FollowFlows {
		t.Errorf("prefs round trip %+v", data.Prefs)
	}
	if len(data.Requests) != 1 || data.Requests[0].Name != "Saved" {
		t.Errorf("requests round trip %+v", data.Requests)
	}
	if len(data.History) != 1 || data.History[0].Status != 200 {
		t.Errorf("history round trip %+v", data.History)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "omniflow.json" {
		t.Errorf("the store left files behind: %v", entries)
	}
}

func TestCorruptStoreIsKeptAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "omniflow.json")
	if err := os.WriteFile(path, []byte("{this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open a corrupt file: %v", err)
	}
	if got := len(s.Data().Requests); got == 0 {
		t.Error("a corrupt file did not fall back to the seed collection")
	}
	raw, err := os.ReadFile(path + ".corrupt")
	if err != nil {
		t.Fatalf("the corrupt file was not kept: %v", err)
	}
	if string(raw) != "{this is not json" {
		t.Errorf("corrupt copy holds %q", raw)
	}
}

func TestDataReturnsACopy(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "omniflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := s.Data()
	before := snapshot.Requests[0].Name
	snapshot.Requests[0].Name = "changed outside"
	if got := s.Data().Requests[0].Name; got != before {
		t.Errorf("store changed through a snapshot: %q", got)
	}
}

func TestMissingPrefsAreRefilled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "omniflow.json")
	if err := os.WriteFile(path, []byte(`{"requests":[],"history":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Prefs().ProxyPort; got != "9090" {
		t.Errorf("proxy port %q, want the default", got)
	}
	if len(s.Data().Requests) != 0 || len(s.Data().History) != 0 {
		t.Error("empty lists gained entries")
	}
}
