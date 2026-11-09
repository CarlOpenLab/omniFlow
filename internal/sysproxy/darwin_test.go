//go:build darwin

package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func darwinFixture(f *fakeRunner) {
	f.set([]string{"networksetup", "-listallnetworkservices"},
		"An asterisk (*) denotes that a network service is disabled.\nWi-Fi\n*Thunderbolt Bridge\nEthernet\n")
	f.set([]string{"networksetup", "-getwebproxy", "Wi-Fi"},
		"Enabled: Yes\nServer: proxy.corp\nPort: 3128\nAuthenticated Proxy Enabled: 0\n")
	f.set([]string{"networksetup", "-getsecurewebproxy", "Wi-Fi"},
		"Enabled: Yes\nServer: proxy.corp\nPort: 3128\nAuthenticated Proxy Enabled: 0\n")
	f.set([]string{"networksetup", "-getautoproxyurl", "Wi-Fi"},
		"URL: http://pac.corp/proxy.pac\nEnabled: Yes\n")
	f.set([]string{"networksetup", "-getproxyautodiscovery", "Wi-Fi"},
		"Auto Proxy Discovery: On\n")

	f.set([]string{"networksetup", "-getwebproxy", "Ethernet"},
		"Enabled: No\nServer: \nPort: 0\nAuthenticated Proxy Enabled: 0\n")
	f.set([]string{"networksetup", "-getsecurewebproxy", "Ethernet"},
		"Enabled: No\nServer: \nPort: 0\nAuthenticated Proxy Enabled: 0\n")
	f.set([]string{"networksetup", "-getautoproxyurl", "Ethernet"},
		"URL: \nEnabled: No\n")
	f.set([]string{"networksetup", "-getproxyautodiscovery", "Ethernet"},
		"Auto Proxy Discovery: Off\n")
}

func TestDarwinSnapshotsServicesAndAppliesTheProxy(t *testing.T) {
	f := newFakeRunner()
	darwinFixture(f)
	ops := newDarwinOps(f)
	ctx := context.Background()

	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	var state darwinState
	if err := json.Unmarshal(snap, &state); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(state.Services) != 2 || state.Services[0].Name != "Wi-Fi" || state.Services[1].Name != "Ethernet" {
		t.Fatalf("services = %+v, want Wi-Fi and Ethernet", state.Services)
	}
	if web := state.Services[0].Web; !web.Enabled || web.Server != "proxy.corp" || web.Port != 3128 || web.Auth {
		t.Fatalf("Wi-Fi web proxy = %+v", web)
	}
	if auto := state.Services[0].AutoURL; !auto.Enabled || auto.URL != "http://pac.corp/proxy.pac" {
		t.Fatalf("Wi-Fi PAC = %+v", auto)
	}
	if !state.Services[0].AutoDiscovery {
		t.Fatal("Wi-Fi auto discovery was not read")
	}
	if state.Services[1].Web.Enabled || state.Services[1].Web.Server != "" {
		t.Fatalf("Ethernet web proxy = %+v, want disabled and empty", state.Services[1].Web)
	}

	f.resetCalls()
	if err := ops.Apply(ctx, snap, 9090, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	wantApply := [][]string{
		{"networksetup", "-setautoproxystate", "Wi-Fi", "off"},
		{"networksetup", "-setproxyautodiscovery", "Wi-Fi", "off"},
		{"networksetup", "-setwebproxy", "Wi-Fi", "127.0.0.1", "9090"},
		{"networksetup", "-setsecurewebproxy", "Wi-Fi", "127.0.0.1", "9090"},
		{"networksetup", "-setautoproxystate", "Ethernet", "off"},
		{"networksetup", "-setproxyautodiscovery", "Ethernet", "off"},
		{"networksetup", "-setwebproxy", "Ethernet", "127.0.0.1", "9090"},
		{"networksetup", "-setsecurewebproxy", "Ethernet", "127.0.0.1", "9090"},
	}
	assertCalls(t, f.calls, wantApply)

	f.resetCalls()
	if err := ops.Restore(ctx, snap); err != nil {
		t.Fatalf("restore: %v", err)
	}
	wantRestore := [][]string{
		{"networksetup", "-setautoproxyurl", "Wi-Fi", "http://pac.corp/proxy.pac"},
		{"networksetup", "-setautoproxystate", "Wi-Fi", "on"},
		{"networksetup", "-setproxyautodiscovery", "Wi-Fi", "on"},
		{"networksetup", "-setwebproxy", "Wi-Fi", "proxy.corp", "3128"},
		{"networksetup", "-setsecurewebproxy", "Wi-Fi", "proxy.corp", "3128"},
		{"networksetup", "-setautoproxystate", "Ethernet", "off"},
		{"networksetup", "-setproxyautodiscovery", "Ethernet", "off"},
		{"networksetup", "-setwebproxy", "Ethernet", "", "0"},
		{"networksetup", "-setwebproxystate", "Ethernet", "off"},
		{"networksetup", "-setsecurewebproxy", "Ethernet", "", "0"},
		{"networksetup", "-setsecurewebproxystate", "Ethernet", "off"},
	}
	assertCalls(t, f.calls, wantRestore)
}

func TestDarwinHTTPOnlyTurnsTheSecureProxyOff(t *testing.T) {
	f := newFakeRunner()
	darwinFixture(f)
	ops := newDarwinOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	f.resetCalls()
	if err := ops.Apply(ctx, snap, 9090, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := f.callsStarting("-setsecurewebproxy"); len(got) != 0 {
		t.Fatalf("http-only apply set a secure proxy: %v", got)
	}
	off := f.callsStarting("-setsecurewebproxystate")
	if len(off) != 2 {
		t.Fatalf("secure proxy state calls = %v, want one per service", off)
	}
	for _, c := range off {
		if c[len(c)-1] != "off" {
			t.Fatalf("secure proxy state = %v, want off", c)
		}
	}
}

func TestDarwinSkipsAuthenticatedProxyServices(t *testing.T) {
	f := newFakeRunner()
	darwinFixture(f)
	f.set([]string{"networksetup", "-getwebproxy", "Wi-Fi"},
		"Enabled: Yes\nServer: proxy.corp\nPort: 3128\nAuthenticated Proxy Enabled: 1\n")
	ops := newDarwinOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	f.resetCalls()
	if err := ops.Apply(ctx, snap, 9090, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := f.callsStarting("-setwebproxy", "Wi-Fi"); len(got) != 0 {
		t.Fatalf("an authenticated service was changed: %v", got)
	}
	if got := f.callsStarting("-setwebproxy", "Ethernet"); len(got) != 1 {
		t.Fatalf("the unauthenticated service was not changed: %v", f.calls)
	}

	f.resetCalls()
	if err := ops.Restore(ctx, snap); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := f.callsStarting("-setwebproxy", "Wi-Fi"); len(got) != 0 {
		t.Fatalf("an authenticated service was restored: %v", got)
	}
}

func TestDarwinAllServicesAuthenticatedIsAnError(t *testing.T) {
	f := newFakeRunner()
	f.set([]string{"networksetup", "-listallnetworkservices"},
		"An asterisk (*) denotes that a network service is disabled.\nWi-Fi\n")
	f.set([]string{"networksetup", "-getwebproxy", "Wi-Fi"},
		"Enabled: Yes\nServer: proxy.corp\nPort: 3128\nAuthenticated Proxy Enabled: 1\n")
	f.set([]string{"networksetup", "-getsecurewebproxy", "Wi-Fi"},
		"Enabled: No\nServer: \nPort: 0\nAuthenticated Proxy Enabled: 0\n")
	f.set([]string{"networksetup", "-getautoproxyurl", "Wi-Fi"}, "URL: \nEnabled: No\n")
	f.set([]string{"networksetup", "-getproxyautodiscovery", "Wi-Fi"}, "Auto Proxy Discovery: Off\n")
	ops := newDarwinOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := ops.Apply(ctx, snap, 9090, true); err == nil {
		t.Fatal("apply succeeded with only authenticated services")
	}
}

func TestDarwinNoServicesIsAnError(t *testing.T) {
	f := newFakeRunner()
	f.set([]string{"networksetup", "-listallnetworkservices"},
		"An asterisk (*) denotes that a network service is disabled.\n")
	ops := newDarwinOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := ops.Apply(ctx, snap, 9090, true); err == nil {
		t.Fatal("apply succeeded with no network services")
	}
}

func TestDarwinCommandFailureIsReported(t *testing.T) {
	f := newFakeRunner()
	darwinFixture(f)
	f.fail([]string{"networksetup", "-setwebproxy", "Ethernet", "127.0.0.1", "9090"},
		errors.New("networksetup failed"))
	ops := newDarwinOps(f)
	ctx := context.Background()
	snap, err := ops.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := ops.Apply(ctx, snap, 9090, true); err == nil {
		t.Fatal("apply succeeded, want the injected failure")
	}
}
