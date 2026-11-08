package main

// The window's frame: what mygo draws around the view, and the controls the
// view draws itself.
//
// The title bar is hidden. On macOS the system's traffic lights are kept at
// the top-left, and the view leaves their room alone. On Windows and Linux the
// system's buttons are taken away and the view draws Windows 11's minimize,
// maximize or restore, and close at the top-right, where those platforms keep
// them: the system's own would cover the card header's controls instead of
// lining up with them.

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

const (
	// The window's controls on Windows and Linux are three flat buttons at
	// the right end of the card's header, where Windows 11 keeps its own.
	windowButtonW     = float32(28)
	windowButtonH     = float32(26)
	windowButtonGap   = float32(2)
	windowButtonRight = float32(uitokens.CardGap + 10)
	windowButtonTop   = float32(uitokens.BarMid) - windowButtonH/2

	// windowButtonsW is the room the three take, and windowButtonsRoom the
	// room a card's header keeps clear for them at its right.
	windowButtonsW    = 3*windowButtonW + 2*windowButtonGap
	windowButtonsRoom = windowButtonsW + 8
)

// controlsAtRight reports whether the view draws the window's controls at the
// top-right corner, as Windows and Linux keep them, rather than leaving them
// to the system at macOS's top-left. It is a variable so a test can draw
// either.
var controlsAtRight = runtime.GOOS != "darwin"

// framedWindow applies the window's frame before it is created: the system's
// controls only where the view does not draw its own.
func framedWindow(o mygo.WindowOptions) mygo.WindowOptions {
	if controlsAtRight {
		o.TitleBarStyle = mygo.TitleBarHidden
		o.HideWindowButtons = true
		return o
	}
	o.TitleBarStyle = mygo.TitleBarHiddenInset
	return o
}

// windowFrame is the part of a window the controls act on. mygo.Window
// satisfies it; a test stands its own in.
type windowFrame interface {
	Minimize()
	Maximize()
	Unmaximize()
	Close()
	IsMaximized() bool
}

// frameOf is the window's frame, or a test's stand-in for it. The app always
// has a real one by the time it draws.
func (a *app) frameOf() windowFrame {
	if a.win != nil {
		return a.win
	}
	return a.frameStub
}

// windowControls draws the window's own controls, and only on Windows and
// Linux: macOS's traffic lights are the system's, so there is nothing to draw
// there. They are drawn in an overlay so they stay above a dialog and above
// everything else the window shows.
func (a *app) windowControls(c *ui.Context) {
	if !controlsAtRight {
		return
	}
	frame := a.frameOf()
	if frame == nil {
		return
	}
	pal := paletteFor(c)
	ui.Overlay(c, func() {
		a.windowButtons(c, pal, frame)
	})
}

// windowButtons draws minimize, maximize or restore and close at the right end
// of the card's header, where Windows draws them: three flat glyphs on the
// app's own surface, the close one turning red under the pointer.
func (a *app) windowButtons(c *ui.Context, pal *uitokens.Palette, frame windowFrame) {
	row := ui.Row(c).Key("window-controls").Absolute().
		Right(windowButtonRight).
		Top(windowButtonTop).
		Height(windowButtonH).Gap(windowButtonGap).AlignItems(ui.Center)
	row.Children(func() {
		windowButton(c, pal, "window-minimize", iconMinimize,
			locales.WindowMinimize(), false, false, frame.Minimize)

		maximized := frame.IsMaximized()
		icon, label := iconMaximize, locales.WindowMaximize()
		if maximized {
			icon, label = iconRestore, locales.WindowRestore()
		}
		windowButton(c, pal, "window-maximize", icon, label, false, true, func() {
			if maximized {
				frame.Unmaximize()
			} else {
				frame.Maximize()
			}
		})

		windowButton(c, pal, "window-close", iconX,
			locales.WindowClose(), true, false, frame.Close)
	})
}

// windowButton is one of the window's buttons on Windows and Linux: a quiet
// glyph that gets a wash under the pointer, and the close one a red face.
func windowButton(c *ui.Context, pal *uitokens.Palette, key string, icon *ui.SVG,
	label string, dangerous, maximize bool, run func()) {
	b := ui.ButtonBase(c).Key(key).Size(windowButtonW, windowButtonH).
		Radius(6).Center().Cursor(ui.CursorPointer).Label(label)
	if maximize {
		// The system's maximize button for the pointer: on Windows 11 it brings
		// the snap layouts while the pointer rests there, and no tooltip covers
		// them.
		b.MaximizeButton()
	} else {
		b.Tooltip(label)
	}
	col := pal.TextMuted
	switch {
	case dangerous && (b.Hovered() || b.Pressed()):
		b.Background(pal.Danger)
		col = ui.RGB(255, 255, 255)
	case b.Pressed():
		b.Background(pal.RowHover.Alpha(0.16))
		col = pal.Text
	case b.Hovered():
		b.Background(pal.RowHover)
		col = pal.Text
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14).TextColor(col)
	})
	if b.Clicked() {
		run()
	}
}
