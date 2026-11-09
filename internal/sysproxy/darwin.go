//go:build darwin

package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// darwinProxy is one service's proxy settings, as networksetup reports them.
type darwinProxy struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
	Auth    bool   `json:"auth"`
}

// darwinAutoURL is a service's proxy auto-config, and whether it was on.
type darwinAutoURL struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

// darwinService is everything OmniFlow may change on one network service.
type darwinService struct {
	Name          string        `json:"name"`
	Web           darwinProxy   `json:"web"`
	Secure        darwinProxy   `json:"secure"`
	AutoURL       darwinAutoURL `json:"autoURL"`
	AutoDiscovery bool          `json:"autoDiscovery"`
}

// darwinState is the snapshot stored in the backup file.
type darwinState struct {
	Services []darwinService `json:"services"`
}

type darwinOps struct {
	run runner
}

func newDarwinOps(r runner) platformOps { return &darwinOps{run: r} }

func (d *darwinOps) Name() string { return "darwin" }

func (d *darwinOps) Snapshot(ctx context.Context) (json.RawMessage, error) {
	out, err := d.run.Run(ctx, "networksetup", "-listallnetworkservices")
	if err != nil {
		return nil, fmt.Errorf("list network services: %w", err)
	}
	state := darwinState{}
	for _, name := range parseServices(out) {
		web, err := d.getProxy(ctx, "-getwebproxy", name)
		if err != nil {
			return nil, err
		}
		secure, err := d.getProxy(ctx, "-getsecurewebproxy", name)
		if err != nil {
			return nil, err
		}
		auto, err := d.getAutoURL(ctx, name)
		if err != nil {
			return nil, err
		}
		discovery, err := d.getAutoDiscovery(ctx, name)
		if err != nil {
			return nil, err
		}
		state.Services = append(state.Services, darwinService{
			Name: name, Web: web, Secure: secure, AutoURL: auto, AutoDiscovery: discovery,
		})
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (d *darwinOps) Apply(ctx context.Context, original json.RawMessage, port int, captureHTTPS bool) error {
	state, err := decodeDarwinState(original)
	if err != nil {
		return err
	}
	portText := strconv.Itoa(port)
	changed := 0
	for _, svc := range state.Services {
		// An authenticated proxy's credentials live in the keychain;
		// networksetup cannot snapshot or restore them, so the whole
		// service is left as it was.
		if svc.Web.Auth || svc.Secure.Auth {
			continue
		}
		if err := d.set(ctx, "-setautoproxystate", svc.Name, "off"); err != nil {
			return err
		}
		if err := d.set(ctx, "-setproxyautodiscovery", svc.Name, "off"); err != nil {
			return err
		}
		if err := d.set(ctx, "-setwebproxy", svc.Name, "127.0.0.1", portText); err != nil {
			return err
		}
		if captureHTTPS {
			if err := d.set(ctx, "-setsecurewebproxy", svc.Name, "127.0.0.1", portText); err != nil {
				return err
			}
		} else if err := d.set(ctx, "-setsecurewebproxystate", svc.Name, "off"); err != nil {
			return err
		}
		changed++
	}
	if changed == 0 {
		return errors.New("no network service could be set: every enabled service uses an authenticated proxy")
	}
	return nil
}

func (d *darwinOps) Restore(ctx context.Context, original json.RawMessage) error {
	state, err := decodeDarwinState(original)
	if err != nil {
		return err
	}
	var errs []error
	for _, svc := range state.Services {
		if svc.Web.Auth || svc.Secure.Auth {
			continue
		}
		if err := d.restoreService(ctx, svc); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// restoreService puts one service's PAC, discovery and proxies back. A failed
// command does not stop the rest: every setting this app changed is tried, so
// one awkward service cannot leave the others on the capture proxy's port.
func (d *darwinOps) restoreService(ctx context.Context, svc darwinService) error {
	var errs []error
	if svc.AutoURL.Enabled && svc.AutoURL.URL != "" {
		if err := d.set(ctx, "-setautoproxyurl", svc.Name, svc.AutoURL.URL); err != nil {
			errs = append(errs, err)
		}
		if err := d.set(ctx, "-setautoproxystate", svc.Name, "on"); err != nil {
			errs = append(errs, err)
		}
	} else if err := d.set(ctx, "-setautoproxystate", svc.Name, "off"); err != nil {
		errs = append(errs, err)
	}
	discovery := "off"
	if svc.AutoDiscovery {
		discovery = "on"
	}
	if err := d.set(ctx, "-setproxyautodiscovery", svc.Name, discovery); err != nil {
		errs = append(errs, err)
	}
	if err := d.restoreProxy(ctx, "-setwebproxy", "-setwebproxystate", svc.Name, svc.Web); err != nil {
		errs = append(errs, err)
	}
	if err := d.restoreProxy(ctx, "-setsecurewebproxy", "-setsecurewebproxystate", svc.Name, svc.Secure); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func decodeDarwinState(raw json.RawMessage) (darwinState, error) {
	var state darwinState
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, fmt.Errorf("read the system proxy backup: %w", err)
	}
	return state, nil
}

func (d *darwinOps) getProxy(ctx context.Context, verb, service string) (darwinProxy, error) {
	out, err := d.run.Run(ctx, "networksetup", verb, service)
	if err != nil {
		return darwinProxy{}, fmt.Errorf("networksetup %s %s: %w", verb, service, err)
	}
	return parseProxy(out)
}

func (d *darwinOps) getAutoURL(ctx context.Context, service string) (darwinAutoURL, error) {
	out, err := d.run.Run(ctx, "networksetup", "-getautoproxyurl", service)
	if err != nil {
		return darwinAutoURL{}, fmt.Errorf("networksetup -getautoproxyurl %s: %w", service, err)
	}
	return parseAutoURL(out), nil
}

func (d *darwinOps) getAutoDiscovery(ctx context.Context, service string) (bool, error) {
	out, err := d.run.Run(ctx, "networksetup", "-getproxyautodiscovery", service)
	if err != nil {
		return false, fmt.Errorf("networksetup -getproxyautodiscovery %s: %w", service, err)
	}
	return parseAutoDiscovery(out), nil
}

func (d *darwinOps) restoreProxy(ctx context.Context, verb, stateVerb, service string, p darwinProxy) error {
	if p.Server == "" {
		// Setting an empty server is how networksetup clears the field; it
		// turns the proxy on, so the state is put back explicitly.
		if err := d.set(ctx, verb, service, "", "0"); err != nil {
			return err
		}
		return d.set(ctx, stateVerb, service, "off")
	}
	if err := d.set(ctx, verb, service, p.Server, strconv.Itoa(p.Port)); err != nil {
		return err
	}
	if !p.Enabled {
		return d.set(ctx, stateVerb, service, "off")
	}
	return nil
}

func (d *darwinOps) set(ctx context.Context, verb string, args ...string) error {
	out, err := d.run.Run(ctx, "networksetup", append([]string{verb}, args...)...)
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("networksetup %s %s: %w (%s)", verb, strings.Join(args, " "), err, detail)
		}
		return fmt.Errorf("networksetup %s %s: %w", verb, strings.Join(args, " "), err)
	}
	return nil
}

// parseServices reads the service names, skipping the legend and the disabled
// services an asterisk marks.
func parseServices(out []byte) []string {
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		services = append(services, line)
	}
	return services
}

func parseProxy(out []byte) (darwinProxy, error) {
	var p darwinProxy
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Enabled":
			p.Enabled = strings.EqualFold(value, "Yes")
		case "Server":
			p.Server = value
		case "Port":
			n, err := strconv.Atoi(value)
			if err != nil {
				return p, fmt.Errorf("read proxy port %q: %w", value, err)
			}
			p.Port = n
		case "Authenticated Proxy Enabled":
			p.Auth = value == "1" || strings.EqualFold(value, "Yes")
		}
	}
	return p, nil
}

func parseAutoURL(out []byte) darwinAutoURL {
	var a darwinAutoURL
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "URL":
			a.URL = value
		case "Enabled":
			a.Enabled = strings.EqualFold(value, "Yes")
		}
	}
	return a
}

func parseAutoDiscovery(out []byte) bool {
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "Auto Proxy Discovery" {
			return strings.EqualFold(strings.TrimSpace(value), "On")
		}
	}
	return false
}
