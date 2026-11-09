//go:build windows

package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const wininetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// windowsValue keeps one registry value, including whether it existed at all:
// restoring an absent value means deleting it, not writing an empty one.
type windowsValue struct {
	Present bool   `json:"present"`
	Kind    string `json:"kind"` // "string" or "dword"
	String  string `json:"string,omitempty"`
	Dword   uint32 `json:"dword,omitempty"`
}

type windowsState struct {
	ProxyEnable   windowsValue `json:"proxyEnable"`
	ProxyServer   windowsValue `json:"proxyServer"`
	ProxyOverride windowsValue `json:"proxyOverride"`
	AutoConfigURL windowsValue `json:"autoConfigURL"`
	AutoDetect    windowsValue `json:"autoDetect"`
}

type windowsOps struct{}

func newWindowsOps() platformOps { return windowsOps{} }

func (windowsOps) Name() string { return "windows" }

func (windowsOps) Snapshot(context.Context) (json.RawMessage, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, wininetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open the WinINet settings: %w", err)
	}
	defer k.Close()

	var state windowsState
	if state.ProxyEnable, err = readDWord(k, "ProxyEnable"); err != nil {
		return nil, err
	}
	if state.ProxyServer, err = readString(k, "ProxyServer"); err != nil {
		return nil, err
	}
	if state.ProxyOverride, err = readString(k, "ProxyOverride"); err != nil {
		return nil, err
	}
	if state.AutoConfigURL, err = readString(k, "AutoConfigURL"); err != nil {
		return nil, err
	}
	if state.AutoDetect, err = readDWord(k, "AutoDetect"); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (windowsOps) Apply(_ context.Context, original json.RawMessage, port int, captureHTTPS bool) error {
	var state windowsState
	if err := json.Unmarshal(original, &state); err != nil {
		return fmt.Errorf("read the system proxy backup: %w", err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, wininetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open the WinINet settings: %w", err)
	}
	defer k.Close()

	// A PAC or auto-detection would take precedence over the static proxy,
	// so both are turned off for the capture and restored from the snapshot.
	if err := k.SetStringValue("AutoConfigURL", ""); err != nil {
		return fmt.Errorf("disable the proxy auto-config: %w", err)
	}
	if err := k.SetDWordValue("AutoDetect", 0); err != nil {
		return fmt.Errorf("disable proxy auto-detection: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", wininetProxyServer(port, captureHTTPS)); err != nil {
		return fmt.Errorf("set the proxy server: %w", err)
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("enable the proxy: %w", err)
	}
	notifyWinINet()
	return nil
}

func (windowsOps) Restore(_ context.Context, original json.RawMessage) error {
	var state windowsState
	if err := json.Unmarshal(original, &state); err != nil {
		return fmt.Errorf("read the system proxy backup: %w", err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, wininetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open the WinINet settings: %w", err)
	}
	defer k.Close()

	values := []struct {
		name  string
		value windowsValue
	}{
		{"ProxyEnable", state.ProxyEnable},
		{"ProxyServer", state.ProxyServer},
		{"ProxyOverride", state.ProxyOverride},
		{"AutoConfigURL", state.AutoConfigURL},
		{"AutoDetect", state.AutoDetect},
	}
	for _, v := range values {
		if err := restoreValue(k, v.name, v.value); err != nil {
			return err
		}
	}
	notifyWinINet()
	return nil
}

func readString(k registry.Key, name string) (windowsValue, error) {
	v, _, err := k.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return windowsValue{}, nil
	}
	if err != nil {
		return windowsValue{}, fmt.Errorf("read %s: %w", name, err)
	}
	return windowsValue{Present: true, Kind: "string", String: v}, nil
}

func readDWord(k registry.Key, name string) (windowsValue, error) {
	v, _, err := k.GetIntegerValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return windowsValue{}, nil
	}
	if err != nil {
		return windowsValue{}, fmt.Errorf("read %s: %w", name, err)
	}
	return windowsValue{Present: true, Kind: "dword", Dword: uint32(v)}, nil
}

func restoreValue(k registry.Key, name string, v windowsValue) error {
	if !v.Present {
		err := k.DeleteValue(name)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
		return nil
	}
	if v.Kind == "dword" {
		if err := k.SetDWordValue(name, v.Dword); err != nil {
			return fmt.Errorf("restore %s: %w", name, err)
		}
		return nil
	}
	if err := k.SetStringValue(name, v.String); err != nil {
		return fmt.Errorf("restore %s: %w", name, err)
	}
	return nil
}

var (
	wininetDLL            = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOption = wininetDLL.NewProc("InternetSetOptionW")
)

const (
	internetOptionRefresh         = 37
	internetOptionSettingsChanged = 39
)

// notifyWinINet makes running programs reread the settings; without it a
// browser may keep using the old proxy until it restarts.
func notifyWinINet() {
	_, _, _ = procInternetSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	_, _, _ = procInternetSetOption.Call(0, internetOptionRefresh, 0, 0)
}
