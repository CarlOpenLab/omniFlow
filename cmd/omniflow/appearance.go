package main

import (
	"github.com/egoist/mygo"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// blur is the strength of the window's material, as prefs store it.
type blur string

const (
	blurNone   blur = "none"   // no material: the ground is painted whole
	blurSubtle blur = "subtle" // the sidebar material, the default
	blurMedium blur = "medium" // the popover material
	blurDeep   blur = "deep"   // HUD on macOS, Acrylic on Windows 11
)

// blurs are the settings' segments, in order.
var blurs = []struct {
	id    blur
	label func() string
}{
	{blurNone, locales.BlurNone},
	{blurSubtle, locales.BlurSubtle},
	{blurMedium, locales.BlurMedium},
	{blurDeep, locales.BlurDeep},
}

// normalizeBlur reads a strength it does not know, from an older file or
// one edited by hand, as the default: a bad value is not worth a window
// without its material.
func normalizeBlur(id string) blur {
	switch b := blur(id); b {
	case blurNone, blurSubtle, blurMedium, blurDeep:
		return b
	}
	return blurSubtle
}

// vibrancy is the window material a strength asks for.
func (b blur) vibrancy() mygo.Vibrancy {
	switch normalizeBlur(string(b)) {
	case blurNone:
		return mygo.VibrancyNone
	case blurMedium:
		return mygo.VibrancyPopover
	case blurDeep:
		return mygo.VibrancyAcrylic
	}
	return mygo.VibrancySidebar
}

// setBlur changes the material at once: it lives on the native side of the
// window, so the window is told, and redrawn.
func (a *app) setBlur(id blur) {
	if normalizeBlur(a.prefs.Blur) == id {
		return
	}
	a.prefs.Blur = string(id)
	a.savePrefs()
	if a.win != nil {
		a.win.SetVibrancy(id.vibrancy())
		a.win.Invalidate()
	}
}

// cardOpacity is the workspace card's opacity, the one layer that has one.
func (a *app) cardOpacity() float64 {
	return uitokens.NormalizeOpacity(a.prefs.CardOpacity)
}

// blurInUse is the material strength the window is showing now.
func (a *app) blurInUse() blur {
	return normalizeBlur(a.prefs.Blur)
}
