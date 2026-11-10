package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/store"
)

// Opening the app is not starting capture: the proxy starts when the person
// starts it. A profile saved when opening used to start it still says so in
// its file, and that stored choice must not start it either.
func TestOpeningTheAppDoesNotStartCapture(t *testing.T) {
	a := newTestApp(t)
	path := filepath.Join(t.TempDir(), "omniflow.json")
	raw := fmt.Sprintf(`{"prefs":{"proxyPort":%q,"proxyAutostart":true}}`, freePort(t))
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write the stored profile: %v", err)
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open the stored profile: %v", err)
	}
	a.store = st
	a.prefs = st.Data().Prefs
	a.proxyPort = a.prefs.ProxyPort

	a.onReady()

	if a.proxyRunning() {
		t.Fatal("opening the app started capture")
	}
}
