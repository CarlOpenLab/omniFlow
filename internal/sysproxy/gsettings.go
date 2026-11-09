package sysproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const gsettingsProxySchema = "org.gnome.system.proxy"

// gsettingsState keeps the raw values gsettings printed, so restoring them
// writes back exactly what the desktop had.
type gsettingsState struct {
	Mode         string `json:"mode"`
	HTTPHost     string `json:"httpHost"`
	HTTPPort     string `json:"httpPort"`
	HTTPSHost    string `json:"httpsHost"`
	HTTPSPort    string `json:"httpsPort"`
	UseSameProxy string `json:"useSameProxy"`
}

type gsettingsOps struct {
	run runner
}

func newGsettingsOps(r runner) platformOps { return &gsettingsOps{run: r} }

func (g *gsettingsOps) Name() string { return "linux" }

func (g *gsettingsOps) Snapshot(ctx context.Context) (json.RawMessage, error) {
	var state gsettingsState
	var err error
	if state.Mode, err = g.get(ctx, gsettingsProxySchema, "mode"); err != nil {
		return nil, err
	}
	if state.HTTPHost, err = g.get(ctx, gsettingsProxySchema+".http", "host"); err != nil {
		return nil, err
	}
	if state.HTTPPort, err = g.get(ctx, gsettingsProxySchema+".http", "port"); err != nil {
		return nil, err
	}
	if state.HTTPSHost, err = g.get(ctx, gsettingsProxySchema+".https", "host"); err != nil {
		return nil, err
	}
	if state.HTTPSPort, err = g.get(ctx, gsettingsProxySchema+".https", "port"); err != nil {
		return nil, err
	}
	if state.UseSameProxy, err = g.get(ctx, gsettingsProxySchema, "use-same-proxy"); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (g *gsettingsOps) Apply(ctx context.Context, original json.RawMessage, port int, captureHTTPS bool) error {
	var state gsettingsState
	if err := json.Unmarshal(original, &state); err != nil {
		return fmt.Errorf("read the system proxy backup: %w", err)
	}
	if err := g.set(ctx, gsettingsProxySchema, "mode", "'manual'"); err != nil {
		return err
	}
	if err := g.set(ctx, gsettingsProxySchema+".http", "host", "'127.0.0.1'"); err != nil {
		return err
	}
	if err := g.set(ctx, gsettingsProxySchema+".http", "port", strconv.Itoa(port)); err != nil {
		return err
	}
	if captureHTTPS {
		if err := g.set(ctx, gsettingsProxySchema+".https", "host", "'127.0.0.1'"); err != nil {
			return err
		}
		if err := g.set(ctx, gsettingsProxySchema+".https", "port", strconv.Itoa(port)); err != nil {
			return err
		}
	} else {
		if err := g.set(ctx, gsettingsProxySchema+".https", "host", "''"); err != nil {
			return err
		}
		if err := g.set(ctx, gsettingsProxySchema+".https", "port", "0"); err != nil {
			return err
		}
	}
	// The schema defaults this to true, which would send HTTPS through the
	// HTTP proxy even when the certificate is not trusted.
	if err := g.set(ctx, gsettingsProxySchema, "use-same-proxy", "false"); err != nil {
		return err
	}
	return nil
}

func (g *gsettingsOps) Restore(ctx context.Context, original json.RawMessage) error {
	var state gsettingsState
	if err := json.Unmarshal(original, &state); err != nil {
		return fmt.Errorf("read the system proxy backup: %w", err)
	}
	values := []struct {
		schema, key, value string
	}{
		{gsettingsProxySchema, "mode", state.Mode},
		{gsettingsProxySchema + ".http", "host", state.HTTPHost},
		{gsettingsProxySchema + ".http", "port", state.HTTPPort},
		{gsettingsProxySchema + ".https", "host", state.HTTPSHost},
		{gsettingsProxySchema + ".https", "port", state.HTTPSPort},
		{gsettingsProxySchema, "use-same-proxy", state.UseSameProxy},
	}
	// Every value is tried even if one fails, so the settings after a failed
	// write are not left pointing at the capture proxy.
	var errs []error
	for _, v := range values {
		if err := g.set(ctx, v.schema, v.key, v.value); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (g *gsettingsOps) get(ctx context.Context, schema, key string) (string, error) {
	out, err := g.run.Run(ctx, "gsettings", "get", schema, key)
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("gsettings get %s %s: %w (%s)", schema, key, err, detail)
		}
		return "", fmt.Errorf("gsettings get %s %s: %w", schema, key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *gsettingsOps) set(ctx context.Context, schema, key, value string) error {
	out, err := g.run.Run(ctx, "gsettings", "set", schema, key, value)
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("gsettings set %s %s: %w (%s)", schema, key, err, detail)
		}
		return fmt.Errorf("gsettings set %s %s: %w", schema, key, err)
	}
	return nil
}
