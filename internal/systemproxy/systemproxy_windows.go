//go:build windows

package systemproxy

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	regKey        = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	stateName     = "system-proxy.json"
	optionRefresh = 37
	optionChanged = 39
)

type saved struct {
	EnablePresent   bool   `json:"enablePresent"`
	Enable          int    `json:"enable"`
	ServerPresent   bool   `json:"serverPresent"`
	Server          string `json:"server"`
	OverridePresent bool   `json:"overridePresent"`
	Override        string `json:"override"`
	OurServer       string `json:"ourServer"`
}

var wininet = syscall.NewLazyDLL("wininet.dll")

// Current reads the system proxy without changing it.
func Current(stateDir string) Status {
	enabledRaw, enabledPresent, err := readValue("ProxyEnable")
	if err != nil {
		return Status{Supported: true, Err: err.Error()}
	}
	server, _, err := readValue("ProxyServer")
	if err != nil {
		return Status{Supported: true, Err: err.Error()}
	}
	enabled := false
	if enabledPresent {
		if n, err := parseDWORD(enabledRaw); err == nil {
			enabled = n != 0
		}
	}
	saved, _ := readSaved(stateDir)
	return Status{
		Supported: true,
		Enabled:   enabled && server != "",
		Ours:      saved != nil && strings.EqualFold(saved.OurServer, server),
		Server:    server,
	}
}

// Enable points the system's HTTP proxy at server, after saving what was
// there. Calling it again with a new server keeps the original snapshot and
// only changes the address OmniFlow owns.
func Enable(stateDir, server string) error {
	snap, err := readSaved(stateDir)
	if err != nil {
		return err
	}
	if snap == nil {
		if snap, err = readSnapshot(); err != nil {
			return err
		}
	}
	snap.OurServer = server
	if err := writeSaved(stateDir, snap); err != nil {
		return err
	}
	if err := writeValue("ProxyEnable", "REG_DWORD", "1"); err != nil {
		return err
	}
	if err := writeValue("ProxyServer", "REG_SZ", server); err != nil {
		return err
	}
	if err := writeValue("ProxyOverride", "REG_SZ", withLocalBypass(snap.Override)); err != nil {
		return err
	}
	notify()
	return nil
}

// withLocalBypass adds the loopback names to the bypass list, so a browser
// pointed at the proxy can still open this machine's own pages.
func withLocalBypass(override string) string {
	have := map[string]bool{}
	for _, part := range strings.Split(override, ";") {
		have[strings.ToLower(strings.TrimSpace(part))] = true
	}
	for _, bypass := range []string{"<local>", "127.0.0.1", "localhost"} {
		if have[bypass] {
			continue
		}
		if strings.TrimSpace(override) != "" {
			override += ";"
		}
		override += bypass
		have[bypass] = true
	}
	return override
}

// Disable puts back what Enable found, and forgets the snapshot.
func Disable(stateDir string) error {
	snap, err := readSaved(stateDir)
	if err != nil || snap == nil {
		return err
	}
	if snap.EnablePresent {
		if err := writeValue("ProxyEnable", "REG_DWORD", strconv.Itoa(snap.Enable)); err != nil {
			return err
		}
	} else if err := deleteValue("ProxyEnable"); err != nil {
		return err
	}
	if snap.ServerPresent {
		if err := writeValue("ProxyServer", "REG_SZ", snap.Server); err != nil {
			return err
		}
	} else if err := deleteValue("ProxyServer"); err != nil {
		return err
	}
	if snap.OverridePresent {
		if err := writeValue("ProxyOverride", "REG_SZ", snap.Override); err != nil {
			return err
		}
	} else if err := deleteValue("ProxyOverride"); err != nil {
		return err
	}
	notify()
	return os.Remove(statePath(stateDir))
}

func readSnapshot() (*saved, error) {
	snap := &saved{}
	var err error
	if snap.Enable, snap.EnablePresent, err = readDWORD("ProxyEnable"); err != nil {
		return nil, err
	}
	if snap.Server, snap.ServerPresent, err = readValue("ProxyServer"); err != nil {
		return nil, err
	}
	if snap.Override, snap.OverridePresent, err = readValue("ProxyOverride"); err != nil {
		return nil, err
	}
	return snap, nil
}

func readDWORD(name string) (int, bool, error) {
	raw, present, err := readValue(name)
	if err != nil || !present {
		return 0, present, err
	}
	n, err := parseDWORD(raw)
	return n, true, err
}

func parseDWORD(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "0x") {
		n, err := strconv.ParseUint(raw[2:], 16, 32)
		return int(n), err
	}
	n, err := strconv.Atoi(raw)
	return n, err
}

func readValue(name string) (string, bool, error) {
	out, err := exec.Command("reg", "query", regKey, "/v", name).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " "), true, nil
		}
	}
	return "", false, nil
}

func writeValue(name, typ, value string) error {
	return exec.Command("reg", "add", regKey, "/v", name, "/t", typ, "/d", value, "/f").Run()
}

func deleteValue(name string) error {
	err := exec.Command("reg", "delete", regKey, "/v", name, "/f").Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil // already absent
	}
	return err
}

func statePath(dir string) string { return filepath.Join(dir, stateName) }

func readSaved(dir string) (*saved, error) {
	raw, err := os.ReadFile(statePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap saved
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func writeSaved(dir string, snap *saved) error {
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(statePath(dir), raw, 0o600)
}

// notify tells WinINet that the settings changed, so open browsers pick
// them up without a restart.
func notify() {
	proc := wininet.NewProc("InternetSetOptionW")
	if proc == nil {
		return
	}
	_, _, _ = proc.Call(0, optionChanged, 0, 0)
	_, _, _ = proc.Call(0, optionRefresh, 0, 0)
}
