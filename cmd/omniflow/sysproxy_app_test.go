package main

import (
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/sysproxy"
)

// freePort asks the kernel for a loopback port and gives it back, so a test
// proxy does not collide with the OmniFlow that may already be running.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release the free port: %v", err)
	}
	return strconv.Itoa(port)
}

type fakeEnable struct {
	port  int
	https bool
}

type fakeSystemProxy struct {
	enables      []fakeEnable
	enableErr    error
	restoreErr   error
	recoverErr   error
	restoreCalls int
	recoverCalls int
	resetCalls   int
}

func (f *fakeSystemProxy) Enable(port int, captureHTTPS bool) error {
	f.enables = append(f.enables, fakeEnable{port: port, https: captureHTTPS})
	return f.enableErr
}

func (f *fakeSystemProxy) Restore() error {
	f.restoreCalls++
	return f.restoreErr
}

func (f *fakeSystemProxy) Recover() error {
	f.recoverCalls++
	return f.recoverErr
}

func (f *fakeSystemProxy) Reset() { f.resetCalls++ }

func trustAs(trusted, known bool) func(string) (bool, bool) {
	return func(string) (bool, bool) { return trusted, known }
}

func withTrust(t *testing.T, trusted, known bool) {
	t.Helper()
	old := systemTrustCheck
	systemTrustCheck = trustAs(trusted, known)
	t.Cleanup(func() { systemTrustCheck = old })
}

func TestStartProxyTakesTheSystemProxy(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = freePort(t)
	a.autoProxy = true
	a.mitm = true
	fake := &fakeSystemProxy{}
	a.sysproxy = fake
	withTrust(t, true, true)
	t.Cleanup(func() { a.prx.Stop() })

	a.startProxy()
	if !a.proxyRunning() {
		t.Fatal("the proxy did not start")
	}
	if len(fake.enables) != 1 {
		t.Fatalf("system proxy enabled %d times, want 1", len(fake.enables))
	}
	if got := fake.enables[0]; got.port <= 0 || !got.https {
		t.Fatalf("enabled port=%d https=%v, want the listening port and HTTPS", got.port, got.https)
	}

	a.stopProxy()
	if fake.restoreCalls != 1 {
		t.Fatalf("restore calls = %d after stop, want 1", fake.restoreCalls)
	}
	if a.proxyRunning() {
		t.Fatal("the proxy is still running after stop")
	}
}

func TestUntrustedCAEnablesHTTPOnly(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = freePort(t)
	a.autoProxy = true
	a.mitm = true
	fake := &fakeSystemProxy{}
	a.sysproxy = fake
	withTrust(t, false, true)
	t.Cleanup(func() { a.prx.Stop() })

	a.startProxy()
	if len(fake.enables) != 1 {
		t.Fatalf("system proxy enabled %d times, want 1", len(fake.enables))
	}
	if fake.enables[0].https {
		t.Fatal("HTTPS was proxied although the Root CA is not trusted")
	}
	a.stopProxy()
}

func TestAutoProxyOffLeavesTheSystemProxyAlone(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = freePort(t)
	a.autoProxy = false
	fake := &fakeSystemProxy{}
	a.sysproxy = fake
	t.Cleanup(func() { a.prx.Stop() })

	a.startProxy()
	if len(fake.enables) != 0 {
		t.Fatalf("system proxy enabled %d times with auto proxy off", len(fake.enables))
	}
	// Starting with auto proxy off may restore a leftover backup; what this
	// test cares about is that stopping gives the machine back.
	fake.restoreCalls = 0
	a.stopProxy()
	if fake.restoreCalls != 1 {
		t.Fatalf("restore calls = %d, want 1 even with auto proxy off", fake.restoreCalls)
	}
}

func TestStopProxyRestoresWithoutARunningProxy(t *testing.T) {
	a := newTestApp(t)
	fake := &fakeSystemProxy{}
	a.sysproxy = fake

	a.stopProxy()
	if fake.restoreCalls != 1 {
		t.Fatalf("restore calls = %d, want 1", fake.restoreCalls)
	}
}

func TestCorruptRecoveryBlocksAutomaticEnableUntilToggled(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = freePort(t)
	a.autoProxy = true
	a.mitm = true
	fake := &fakeSystemProxy{recoverErr: sysproxy.ErrCorruptBackup}
	a.sysproxy = fake
	withTrust(t, true, true)
	t.Cleanup(func() { a.prx.Stop() })

	a.recoverSystemProxy()
	if !a.sysproxyBlocked {
		t.Fatal("a corrupt backup did not block automatic control")
	}
	if a.notice == "" {
		t.Fatal("a corrupt backup was not reported")
	}

	a.startProxy()
	if len(fake.enables) != 0 {
		t.Fatalf("system proxy enabled %d times while blocked", len(fake.enables))
	}

	a.setAutoProxy(true)
	if fake.resetCalls != 1 {
		t.Fatalf("reset calls = %d after an explicit toggle, want 1", fake.resetCalls)
	}
	if a.sysproxyBlocked {
		t.Fatal("the explicit toggle did not clear the block")
	}
	if len(fake.enables) != 1 {
		t.Fatalf("system proxy enabled %d times after the toggle, want 1", len(fake.enables))
	}
	a.stopProxy()
}

func TestRecoverFailureIsReportedButDoesNotBlock(t *testing.T) {
	a := newTestApp(t)
	fake := &fakeSystemProxy{recoverErr: errors.New("could not restore")}
	a.sysproxy = fake

	a.recoverSystemProxy()
	if a.sysproxyBlocked {
		t.Fatal("a recover failure blocked automatic control")
	}
	if a.notice == "" {
		t.Fatal("a recover failure was not reported")
	}
}

func TestSetAutoProxyPersistsThePreference(t *testing.T) {
	a := newTestApp(t)
	fake := &fakeSystemProxy{}
	a.sysproxy = fake

	a.setAutoProxy(false)
	if !a.prefs.DisableAutoProxy {
		t.Fatal("turning auto proxy off did not persist")
	}
	if a.autoProxy {
		t.Fatal("turning auto proxy off did not take effect")
	}
	if fake.restoreCalls != 1 {
		t.Fatalf("restore calls = %d, want 1", fake.restoreCalls)
	}

	a.setAutoProxy(true)
	if a.prefs.DisableAutoProxy {
		t.Fatal("turning auto proxy on did not persist")
	}
	if !a.autoProxy {
		t.Fatal("turning auto proxy on did not take effect")
	}
}
