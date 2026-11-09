//go:build windows

package systemproxy

import (
	"strings"
	"testing"
)

func TestParseDWORD(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
		ok   bool
	}{
		{"0x1", 1, true},
		{"1", 1, true},
		{"0", 0, true},
		{"nope", 0, false},
	} {
		got, err := parseDWORD(tc.raw)
		if (err == nil) != tc.ok || (tc.ok && got != tc.want) {
			t.Errorf("parseDWORD(%q) = %d, %v; want %d, ok=%v", tc.raw, got, err, tc.want, tc.ok)
		}
	}
}

func TestSavedSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &saved{
		EnablePresent: true, Enable: 0,
		ServerPresent: true, Server: "proxy.example:8080",
		OverridePresent: true, Override: "<local>",
		OurServer: "127.0.0.1:9090",
	}
	if err := writeSaved(dir, want); err != nil {
		t.Fatalf("write saved: %v", err)
	}
	got, err := readSaved(dir)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if got == nil || *got != *want {
		t.Errorf("saved = %+v, want %+v", got, want)
	}
}

func TestCurrentReadsTheSystemProxy(t *testing.T) {
	st := Current(t.TempDir())
	if !st.Supported {
		t.Error("the Windows system proxy is reported unsupported")
	}
	if st.Err != "" {
		t.Errorf("reading the system proxy failed: %s", st.Err)
	}
}

func TestWithLocalBypassAddsEachNameOnce(t *testing.T) {
	got := withLocalBypass("*.example.com")
	for _, want := range []string{"*.example.com", "<local>", "127.0.0.1", "localhost"} {
		if !strings.Contains(got, want) {
			t.Errorf("bypass list %q lacks %q", got, want)
		}
	}
	again := withLocalBypass(got)
	if again != got {
		t.Errorf("a second pass changed the list: %q -> %q", got, again)
	}
}
