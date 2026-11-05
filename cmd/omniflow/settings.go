package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// settingsDialog is the one place appearance and capture are configured.
func (a *app) settingsDialog(c *ui.Context, pal *uitokens.Palette) {
	ui.DialogBase(c, &a.settingsOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(500).Padding(20).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))

		head := ui.Row(c).Height(32).Gap(8)
		head.Children(func() {
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "Settings").FontSize(16).FontWeight(700).TextColor(pal.Text)
				ui.Text(c, "Appearance, capture and the certificate").FontSize(10.5).
					TextColor(pal.TextFaint)
			})
			if b := iconButton(c, pal, iconX, "Close Settings", 26); b.Clicked() {
				a.settingsOpen = false
			}
		})
		ui.Box(c).Height(12)

		// Appearance
		a.settingsLabel(c, pal, "Appearance")
		mode := themeMode(a.prefs.Theme)
		labels := make([]string, len(themes))
		current := 0
		for i, t := range themes {
			labels[i] = t.label
			if t.mode == mode {
				current = i
			}
		}
		tidx := current
		themeRow := ui.Row(c).Height(30).Gap(10)
		themeRow.Children(func() {
			ui.Text(c, "Theme").FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			segmented(c, pal, &tidx, labels)
		})
		if tidx != current && tidx >= 0 && tidx < len(themes) {
			a.setTheme(themes[tidx].mode)
		}

		// Language: the system's, or one of the languages the app is
		// written in, each named in itself.
		langTags := []string{"", "en", "zh-Hans"}
		langLabels := []string{
			locales.LanguageSystem(),
			i18n.Autonym("en"),
			i18n.Autonym("zh-Hans"),
		}
		langNow := 0
		for i, tag := range langTags {
			if tag == normalizeLanguage(a.prefs.Language) {
				langNow = i
			}
		}
		lidx := langNow
		langRow := ui.Row(c).Height(30).Gap(10)
		langRow.Children(func() {
			ui.Text(c, locales.SettingsLanguage()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			segmented(c, pal, &lidx, langLabels)
		})
		if lidx != langNow && lidx >= 0 && lidx < len(langTags) {
			a.setLanguage(langTags[lidx])
		}
		ui.Box(c).Height(4)

		// The two layers a person can steer: how opaque the card is, and how
		// strong the material behind it is. Both repaint the window as they
		// change, with no Done to press.
		opacity := a.cardOpacity()
		opacityRow := ui.Row(c).Height(30).Gap(10)
		opacityRow.Children(func() {
			ui.Text(c, "Card opacity").FontSize(11.5).TextColor(pal.TextMuted).Width(96).Shrink(0)
			ui.Slider(c.Key("settings-opacity"), &opacity, uitokens.OpacityMin, uitokens.OpacityMax).
				Step(0.05).Grow(1).MinWidth(0).Label("Card opacity")
			ui.Text(c, fmt.Sprintf("%.0f%%", opacity*100)).Width(38).Shrink(0).FontSize(11).
				Font("monospace").TextColor(pal.TextMuted).TextAlign(ui.End)
		})
		if opacity != a.cardOpacity() {
			a.prefs.CardOpacity = opacity
			a.savePrefs()
			if a.win != nil {
				a.win.Invalidate()
			}
		}

		blurLabels := make([]string, len(blurs))
		blurNow := 0
		for i, b := range blurs {
			blurLabels[i] = b.label
			if b.id == a.blurInUse() {
				blurNow = i
			}
		}
		bidx := blurNow
		blurRow := ui.Row(c).Height(30).Gap(10)
		blurRow.Children(func() {
			ui.Text(c, "Material").FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			segmented(c, pal, &bidx, blurLabels)
		})
		if bidx != blurNow && bidx >= 0 && bidx < len(blurs) {
			a.setBlur(blurs[bidx].id)
		}
		ui.Box(c).Height(4)

		// One background: the window's material is the system's own, not a
		// palette of washes. The row shows how it reads in this appearance.
		stops := uitokens.Acrylic.Light
		if pal.Dark {
			stops = uitokens.Acrylic.Dark
		}
		bg := ui.Row(c).Height(50).Padding(0, 10).Gap(10).Radius(10).Background(pal.Control).
			Border(1, pal.Edge)
		bg.Children(func() {
			chip := ui.Column(c).Width(54).Height(32).Shrink(0).Radius(7).Clip().Border(1, pal.Edge)
			chip.Gradient(ui.Hex(stops[0]), ui.Hex(stops[len(stops)-1]), 135)
			chip.Children(func() {
				ui.Box(c).Width(24).Height(14).Radius(4).Margin(9, 0, 0, 15).
					Background(pal.Card.Alpha(0.90)).Shadow(0, 1, 2, 0, pal.CardShadow)
			})
			ui.Column(c).Gap(1).Justify(ui.Center).Children(func() {
				ui.Text(c, "Acrylic background").FontSize(11.5).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "The system's frosted material shows through the window").FontSize(10).
					TextColor(pal.TextFaint)
			})
		})
		ui.Box(c).Height(14)

		// Capture
		a.settingsLabel(c, pal, "Capture")
		portRow := ui.Row(c).Height(30).Gap(10)
		portRow.Children(func() {
			ui.Text(c, "Proxy port").FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			hint, hintColor := "Enter to apply", pal.TextFaint
			if !validPort(a.proxyPort) {
				hint, hintColor = errPort, pal.Danger
			}
			ui.Text(c, hint).FontSize(10).TextColor(hintColor)
			in := ui.TextInput(c.Key("settings-port"), &a.proxyPort).Width(86).Height(26).
				FontSize(11.5).Font("monospace").Padding(0, 8).Background(pal.CodeBg).
				Border(1, pal.Edge)
			if !validPort(a.proxyPort) {
				in.Border(1, pal.Danger)
			}
			if in.Submitted() || in.Changed() {
				a.setProxyPort(in.Submitted())
			}
		})
		timeoutSec, timeoutErr := strconv.Atoi(strings.TrimSpace(a.timeoutText))
		timeoutOK := timeoutErr == nil && timeoutSec >= 1 && timeoutSec <= 600
		timeoutRow := ui.Row(c).Height(30).Gap(10)
		timeoutRow.Children(func() {
			ui.Text(c, "Timeout").FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			ui.Text(c, "seconds").FontSize(10).TextColor(pal.TextFaint)
			in := ui.TextInput(c.Key("settings-timeout"), &a.timeoutText).Width(64).Height(26).
				FontSize(11.5).Font("monospace").Padding(0, 8).Background(pal.CodeBg).
				Border(1, pal.Edge).Label("Timeout in seconds")
			if !timeoutOK {
				in.Border(1, pal.Danger)
			}
			if in.Changed() || in.Submitted() {
				if timeoutOK && a.prefs.TimeoutSec != timeoutSec {
					a.prefs.TimeoutSec = timeoutSec
					a.savePrefs()
				}
			}
		})
		a.settingSwitch(c, pal, "Stop at redirects", &a.prefs.StopAtRedirects)
		a.settingSwitch(c, pal, "Skip certificate verification", &a.prefs.SkipTLSVerify)
		a.settingSwitch(c, pal, "Start the proxy when OmniFlow opens", &a.prefs.ProxyAutostart)
		a.settingSwitch(c, pal, "Decrypt HTTPS with the Root CA", &a.mitm)
		if a.mitm != a.prefs.DecryptHTTPS {
			a.prefs.DecryptHTTPS = a.mitm
			a.savePrefs()
			if a.prx != nil {
				a.prx.SetMITM(a.mitm)
			}
		}
		ui.Box(c).Height(6)
		caRow := ui.Row(c).Height(30).Gap(8)
		caRow.Children(func() {
			ui.Icon(c, iconShield).Size(13, 13).TextColor(pal.TextMuted)
			ui.Text(c, "Root CA Certificate").FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			if ui.Button(c.Key("settings-ca"), "Manage…").Clicked() {
				a.caOpen = true
			}
		})
		ui.Box(c).Height(10)

		// Data
		a.settingsLabel(c, pal, "Data")
		a.settingSwitch(c, pal, "Keep request history", &a.prefs.KeepHistory)
		dataRow := ui.Row(c).Height(30).Gap(8)
		dataRow.Children(func() {
			ui.Icon(c, iconFolder).Size(13, 13).TextColor(pal.TextMuted)
			ui.Column(c).Gap(0).Grow(1).MinWidth(0).Justify(ui.Center).Children(func() {
				ui.Text(c, "Collection, history and certificate").FontSize(11.5).TextColor(pal.TextMuted)
				ui.Text(c, shortenPath(a.dataDir(), 56)).FontSize(10).Font("monospace").
					TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			})
			if ui.Button(c.Key("settings-data"), "Show in Finder").Clicked() {
				a.revealData()
			}
		})
		ui.Box(c).Height(10)

		// About
		a.settingsLabel(c, pal, "About")
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, "OmniFlow "+appVersion()+" · drawn natively on the GPU, with no web view").
				FontSize(10.5).TextColor(pal.TextFaint).Grow(1).LineHeight(1.4)
		})
		ui.Box(c).Height(16)
		ui.Row(c).Height(32).Justify(ui.End).Children(func() {
			if ui.PrimaryButton(c.Key("settings-done"), "Done").Clicked() {
				a.settingsOpen = false
			}
		})
	})
}

func (a *app) settingsLabel(c *ui.Context, pal *uitokens.Palette, text string) {
	ui.Row(c).Height(26).Children(func() {
		ui.Text(c, strings.ToUpper(text)).FontSize(9.5).FontWeight(750).LetterSpacing(0.8).
			TextColor(pal.TextFaint)
	})
}

func (a *app) settingSwitch(c *ui.Context, pal *uitokens.Palette, text string, value *bool) {
	before := *value
	row := ui.Row(c).Height(30).Gap(10)
	row.Children(func() {
		ui.Text(c, text).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
		ui.Switch(c.Key("switch:"+text), value).Label(text)
	})
	if *value != before {
		a.savePrefs()
	}
}

// caDialog explains the one certificate OmniFlow asks a person to trust,
// and offers the file to install.
func (a *app) caDialog(c *ui.Context, pal *uitokens.Palette) {
	// Ask whether this Mac trusts the certificate when the dialog opens,
	// and only then: the answer changes only when a person installs it.
	if a.caOpen && !a.caWasOpen {
		a.checkTrust()
	}
	a.caWasOpen = a.caOpen
	ui.DialogBase(c, &a.caOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(540).Padding(22).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))

		head := ui.Row(c).Height(34).Gap(10)
		head.Children(func() {
			ui.Box(c).Size(34, 34).Radius(9).Center().Background(pal.AccentSoft).Children(func() {
				ui.Icon(c, iconShield).Size(18, 18).TextColor(pal.Accent)
			})
			ui.Column(c).Gap(0).Grow(1).Justify(ui.Center).Children(func() {
				ui.Text(c, "Root CA Certificate").FontSize(15.5).FontWeight(700).TextColor(pal.Text)
				ui.Text(c, "The one trust OmniFlow needs to read HTTPS").FontSize(10.5).
					TextColor(pal.TextFaint)
			})
			if b := iconButton(c, pal, iconX, "Close Certificate Dialog", 26); b.Clicked() {
				a.caOpen = false
			}
		})
		ui.Box(c).Height(14)
		ui.Text(c, "OmniFlow signs a certificate for each HTTPS host on the fly, with a root authority created on this machine. Installing that root in a device's trust store is what lets the inspector read encrypted traffic. Only install it on devices you control.").
			FontSize(11.5).TextColor(pal.TextMuted).LineHeight(1.5)
		ui.Box(c).Height(12)

		steps := []struct{ n, title, body string }{
			{"1", "Save the certificate", "Write the root certificate to a file you can reach from the device."},
			{"2", "Install it", "macOS: double-click it in Keychain Access and set Always Trust. Windows: import it into Trusted Root Certification Authorities. iOS: install the profile, then enable full trust in Settings → General → About."},
			{"3", "Point the device at the proxy", "Set the HTTP proxy to 127.0.0.1:" + a.proxyPort + " on this machine, or to this Mac's address from another device."},
			{"4", "Capture", "Requests appear in the Inspector as they pass. Remove the certificate when you are done."},
		}
		for _, s := range steps {
			step := s
			row := ui.Row(c).AlignItems(ui.Start).Gap(9).Padding(0, 0, 6, 0)
			row.Children(func() {
				ui.Box(c).Size(18, 18).Shrink(0).Radius(9).Center().Background(pal.Chip).
					Children(func() {
						ui.Text(c, step.n).FontSize(9).FontWeight(750).TextColor(pal.TextMuted)
					})
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, step.title).FontSize(11.5).FontWeight(650).TextColor(pal.Text)
					ui.Text(c, step.body).FontSize(10.5).TextColor(pal.TextFaint).LineHeight(1.4)
				})
			})
		}
		ui.Box(c).Height(6)
		path := "Unavailable"
		if a.ca != nil {
			path = a.ca.CertPath()
		}
		pathRow := ui.Row(c).Height(34).Gap(8).Padding(0, 10).Radius(8).Background(pal.CodeBg).
			Border(1, pal.CodeEdge)
		pathRow.Children(func() {
			ui.Icon(c, iconKey).Size(12, 12).TextColor(pal.TextFaint)
			ui.Text(c, shortenPath(path, 64)).Font("monospace").FontSize(10).
				TextColor(pal.TextMuted).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			if b := iconButton(c, pal, iconCopy, "Copy Certificate Path", 24); b.Clicked() {
				a.copyText(c, "Certificate path copied", path)
			}
		})
		ui.Box(c).Height(8)
		trustRow := ui.Row(c).Height(24).Gap(8)
		trustRow.Children(func() {
			col, text := pal.TextFaint, "Trust on this Mac has not been checked"
			switch a.caTrust {
			case trustChecking:
				col, text = pal.TextMuted, "Checking whether this Mac trusts the certificate…"
			case trustYes:
				col, text = pal.Success, "Trusted on this Mac"
			case trustNo:
				col, text = pal.Warning, "Not trusted on this Mac · HTTPS will warn until it is installed"
			}
			statusDot(c, col, 6)
			ui.Text(c, text).FontSize(10.5).TextColor(col).Grow(1).SingleLine().Ellipsis("…")
			if a.caTrust != trustChecking {
				if ui.Button(c.Key("ca-recheck"), "Check Again").Clicked() {
					a.checkTrust()
				}
			}
		})
		ui.Box(c).Height(10)
		ui.Row(c).Height(32).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c.Key("ca-reveal"), "Reveal in Finder").Clicked() {
				a.revealCA()
			}
			if ui.Button(c.Key("ca-cancel"), "Done").Clicked() {
				a.caOpen = false
			}
			if ui.PrimaryButton(c.Key("ca-save"), "Save Certificate…").Clicked() {
				a.exportCA()
			}
		})
	})
}

func shortenPath(p string, n int) string {
	if len(p) <= n {
		return p
	}
	return "…" + p[len(p)-n+1:]
}
