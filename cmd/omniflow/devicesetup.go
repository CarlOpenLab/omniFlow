package main

// Device setup lets a phone or another computer on the same network use the
// capture proxy. The app serves a small page on the LAN with the certificate
// to install and the proxy address to enter; the page needs no certificate
// itself, because it is opened before the proxy is configured.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// lanIP is the first non-loopback IPv4 address of an interface that is up,
// the address a device on the same network can reach.
func lanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	fallback := ""
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallback == "" {
				fallback = ip.String()
			}
		}
	}
	return fallback
}

func (a *app) setLANAccess(on bool) {
	if on == a.lanAccess {
		if on {
			if err := a.ensureSetupServer(); err != nil {
				a.setupErr = err.Error()
			}
		}
		return
	}
	if on && lanIP() == "" {
		a.setNotice(locales.NoticeLANNoAddress())
		return
	}
	a.lanAccess = on
	a.prefs.LANAccess = on
	a.savePrefs()
	if on {
		if err := a.ensureSetupServer(); err != nil {
			a.setupErr = err.Error()
			a.setNotice(locales.NoticeLANSetupFailed(err.Error()))
		} else {
			a.setupErr = ""
		}
		if a.proxyRunning() {
			a.restartProxy()
		}
		a.setNotice(locales.NoticeLANEnabled(a.lanProxyAddr()))
		return
	}
	a.stopSetupServer()
	a.setupErr = ""
	if a.proxyRunning() {
		a.restartProxy()
	}
	a.setNotice(locales.NoticeLANDisabled())
}

// lanProxyAddr is the address a device enters as its HTTP proxy.
func (a *app) lanProxyAddr() string {
	ip := lanIP()
	if ip == "" {
		ip = "127.0.0.1"
	}
	return ip + ":" + a.proxyPort
}

// ensureSetupServer starts the LAN page if it is not running. Its URL carries
// a random token so the certificate is not served to every request that finds
// the port.
func (a *app) ensureSetupServer() error {
	if a.setupSrv != nil {
		return nil
	}
	if a.ca == nil {
		return fmt.Errorf("the certificate is unavailable")
	}
	token, err := setupToken()
	if err != nil {
		return err
	}
	ip := lanIP()
	if ip == "" {
		return fmt.Errorf("no local network address")
	}

	var ln net.Listener
	port := 0
	for candidate := 9091; candidate <= 9099; candidate++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidate))
		if err == nil {
			port = candidate
			break
		}
	}
	if ln == nil {
		return err
	}

	mux := http.NewServeMux()
	base := "/setup/" + token
	mux.HandleFunc(base, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != base {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(a.setupPage(base)))
	})
	mux.HandleFunc(base+"/ca.pem", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", `attachment; filename="omniflow-ca.pem"`)
		_, _ = w.Write(a.ca.CertPEM())
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: time.Minute}
	a.setupSrv = srv
	a.setupURL = fmt.Sprintf("http://%s:%d%s", ip, port, base)
	go func() { _ = srv.Serve(ln) }()
	return nil
}

func (a *app) stopSetupServer() {
	if a.setupSrv == nil {
		return
	}
	_ = a.setupSrv.Close()
	a.setupSrv = nil
	a.setupURL = ""
}

func setupToken() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// setupPage is the page the device opens: the certificate first, then the
// proxy address, in the app's two languages.
func (a *app) setupPage(path string) string {
	ip := lanIP()
	addr := html.EscapeString(ip + ":" + a.proxyPort)
	ca := html.EscapeString(path + "/ca.pem")
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>OmniFlow · Device Setup</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0; padding: 28px 20px 40px; font: 15px/1.55 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f3f5f9; color: #1e2027; }
  main { max-width: 620px; margin: 0 auto; }
  .card { background: #fff; border: 1px solid #e0e3ea; border-radius: 14px; padding: 20px 22px; margin: 14px 0; }
  h1 { font-size: 22px; margin: 0 0 4px; }
  h2 { font-size: 14px; margin: 0 0 8px; }
  p { margin: 6px 0; color: #5d6270; }
  code, a { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  code { background: #f0f2f6; border-radius: 5px; padding: 2px 6px; }
  a { color: #2f6fed; }
  ol { padding-left: 20px; margin: 8px 0; }
  li { margin: 7px 0; }
  .warn { color: #9a5b12; background: #fff7e8; border-color: #f2ddb7; }
  @media (prefers-color-scheme: dark) {
    body { background: #14151a; color: #eef0f6; }
    .card { background: #1d1f26; border-color: #2c303a; }
    p { color: #a7adba; }
    code { background: #262a33; }
    a { color: #7fa6ff; }
    .warn { color: #e4bb79; background: #2a2319; border-color: #4a3a22; }
  }
</style>
</head>
<body><main>
  <h1>OmniFlow Device Setup</h1>
  <p>Same Wi-Fi. Install the certificate, then point the device's HTTP proxy here.</p>
  <div class="card">
    <h2>1 · Certificate</h2>
    <p><a href="%s">Download the OmniFlow Root CA</a></p>
    <p><b>iPhone / iPad:</b> open the link, allow the profile, then Settings → General → VPN &amp; Device Management → OmniFlow Root CA → Install. Finally Settings → General → About → Certificate Trust Settings, and turn OmniFlow on.</p>
    <p><b>Android:</b> open the link, then Settings → Security → Encryption &amp; credentials → Install a certificate → CA certificate, and choose the downloaded file.</p>
  </div>
  <div class="card">
    <h2>2 · Proxy</h2>
    <p>In the Wi-Fi network's HTTP proxy settings choose <b>Manual</b>:</p>
    <p>Server <code>%s</code> · Port <code>%s</code></p>
  </div>
  <div class="card">
    <h2>3 · Capture</h2>
    <p>Press <b>Start Capture</b> in OmniFlow, then open any app on the device. Its traffic appears in the Inspector.</p>
  </div>
  <div class="card warn">
    <p>Only use this on a network you trust. While LAN access is on, other devices on this network can reach the proxy.</p>
  </div>
</main></body></html>`, ca, addr, html.EscapeString(a.proxyPort))
}

func (a *app) deviceSetupDialog(c *ui.Context, pal *uitokens.Palette) {
	ui.DialogBase(c, &a.deviceSetupOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(620).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).Clip().
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08)).Padding(20)
		panel.Children(func() {
			ui.Row(c).Height(34).Gap(8).Children(func() {
				ui.Box(c).Size(34, 34).Radius(9).Center().Background(pal.AccentSoft).Children(func() {
					ui.Icon(c, iconGlobe).Size(18, 18).TextColor(pal.Accent)
				})
				ui.Column(c).Gap(0).Grow(1).Justify(ui.Center).Children(func() {
					ui.Text(c, locales.DeviceSetupTitle()).FontSize(15.5).FontWeight(700).TextColor(pal.Text)
					ui.Text(c, locales.DeviceSetupSubtitle()).FontSize(10.5).TextColor(pal.TextFaint)
				})
				if b := iconButton(c, pal, iconX, locales.CommonCancel(), 26); b.Clicked() {
					a.deviceSetupOpen = false
				}
			})

			ui.Box(c).Height(10)
			ui.Row(c).Height(26).Gap(8).Children(func() {
				ui.Text(c, locales.SettingsLANAccess()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
				lan := a.lanAccess
				ui.Switch(c.Key(fmt.Sprintf("lan-access:%t", a.lanAccess)), &lan).Label(locales.SettingsLANAccess())
				if lan != a.lanAccess {
					a.setLANAccess(lan)
				}
			})
			if !a.lanAccess {
				ui.Text(c, locales.DeviceSetupLANHint()).FontSize(10.5).TextColor(pal.TextFaint).
					LineHeight(1.45)
				return
			}

			ui.Box(c).Height(6)
			addr := a.lanProxyAddr()
			ui.Row(c).Height(30).Gap(8).Children(func() {
				ui.Text(c, locales.DeviceSetupProxy()).FontSize(10.5).TextColor(pal.TextFaint).Width(58)
				ui.Text(c, addr).Font("monospace").FontSize(11.5).TextColor(pal.Text).Grow(1)
				if b := iconButton(c, pal, iconCopy, locales.DeviceSetupCopyAddr(), 22); b.Clicked() {
					a.copyText(c, locales.DeviceSetupAddrCopied(), addr)
				}
			})
			ui.Row(c).Height(30).Gap(8).Children(func() {
				ui.Text(c, locales.DeviceSetupPage()).FontSize(10.5).TextColor(pal.TextFaint).Width(58)
				page := a.setupURL
				if page == "" {
					page = a.setupErr
				}
				ui.Text(c, page).Font("monospace").FontSize(10.5).TextColor(pal.TextMuted).
					Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				if b := iconButton(c, pal, iconCopy, locales.DeviceSetupCopyPage(), 22); b.Clicked() {
					a.copyText(c, locales.DeviceSetupPageCopied(), a.setupURL)
				}
			})
			ui.Box(c).Height(8)
			steps := []struct{ n, text string }{
				{"1", locales.DeviceSetupStep1(a.setupURL)},
				{"2", locales.DeviceSetupStep2()},
				{"3", locales.DeviceSetupStep3(a.lanProxyAddr())},
			}
			for _, s := range steps {
				step := s
				ui.Row(c).AlignItems(ui.Start).Gap(8).Padding(0, 0, 4, 0).Children(func() {
					ui.Box(c).Size(18, 18).Shrink(0).Radius(9).Center().Background(pal.Chip).Children(func() {
						ui.Text(c, step.n).FontSize(9).FontWeight(750).TextColor(pal.TextMuted)
					})
					ui.Text(c, step.text).FontSize(10.5).TextColor(pal.TextMuted).Grow(1).LineHeight(1.45)
				})
			}
			ui.Box(c).Height(6)
			ui.Text(c, locales.DeviceSetupWarning()).FontSize(10.5).TextColor(pal.Warning).LineHeight(1.45)
			ui.Box(c).Height(10)
			ui.Row(c).Height(30).Gap(8).Justify(ui.End).Children(func() {
				if !a.proxyRunning() {
					if ui.PrimaryButton(c.Key("device-start"), locales.InspectorStartCapture()).Clicked() {
						a.startProxy()
					}
				}
				if ui.Button(c.Key("device-done"), locales.CommonDone()).Clicked() {
					a.deviceSetupOpen = false
				}
			})
		})
	})
}
