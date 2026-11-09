package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func gsettingsFixture(f *fakeRunner) {
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy", "mode"}, "'manual'")
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy.http", "host"}, "'proxy.corp'")
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy.http", "port"}, "8080")
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy.https", "host"}, "'secure.corp'")
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy.https", "port"}, "8443")
	f.set([]string{"gsettings", "get", "org.gnome.system.proxy", "use-same-proxy"}, "true")
}

func TestGsettingsSnapshotsAndApplies(t *testing.T) {
	f := newFakeRunner()
	gsettingsFixture(f)
	ops := newGsettingsOps(f)
	ctx := context.Background()

	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	var state gsettingsState
	if err := json.Unmarshal(snap, &state); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if state.Mode != "'manual'" || state.HTTPHost != "'proxy.corp'" || state.HTTPPort != "8080" ||
		state.HTTPSHost != "'secure.corp'" || state.HTTPSPort != "8443" || state.UseSameProxy != "true" {
		t.Fatalf("state = %+v", state)
	}

	f.resetCalls()
	if err := ops.Apply(ctx, snap, 9090, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "'manual'"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "'127.0.0.1'"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", "9090"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "'127.0.0.1'"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", "9090"},
		{"gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", "false"},
	}
	assertCalls(t, f.calls, want)

	f.resetCalls()
	if err := ops.Restore(ctx, snap); err != nil {
		t.Fatalf("restore: %v", err)
	}
	wantRestore := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "'manual'"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "'proxy.corp'"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", "8080"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "'secure.corp'"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", "8443"},
		{"gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", "true"},
	}
	assertCalls(t, f.calls, wantRestore)
}

func TestGsettingsHTTPOnlyClearsTheSecureProxy(t *testing.T) {
	f := newFakeRunner()
	gsettingsFixture(f)
	ops := newGsettingsOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	f.resetCalls()
	if err := ops.Apply(ctx, snap, 9090, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	host := f.callsStarting("set", "org.gnome.system.proxy.https", "host")
	if len(host) != 1 || host[0][len(host[0])-1] != "''" {
		t.Fatalf("https host calls = %v, want one empty value", host)
	}
	port := f.callsStarting("set", "org.gnome.system.proxy.https", "port")
	if len(port) != 1 || port[0][len(port[0])-1] != "0" {
		t.Fatalf("https port calls = %v, want 0", port)
	}
}

func TestGsettingsMissingReportsAnError(t *testing.T) {
	f := newFakeRunner()
	f.fail([]string{"gsettings", "get", "org.gnome.system.proxy", "mode"},
		errors.New("exec: gsettings: executable file not found in $PATH"))
	ops := newGsettingsOps(f)
	if _, err := ops.Snapshot(context.Background()); err == nil {
		t.Fatal("snapshot succeeded without gsettings")
	}
}
