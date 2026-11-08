package main

// The window's frame: macOS keeps its traffic lights and the view lays out
// around the room they report; Windows and Linux take the system's buttons away
// and the view draws its own at the top-right.

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// fakeWindowFrame is the window the controls act on in a test: a headless tester
// has no real one, so it counts the calls instead.
type fakeWindowFrame struct {
	minimized  int
	maximized  int
	restored   int
	closed     int
	stillMaxed bool
}

func (f *fakeWindowFrame) Minimize()         { f.minimized++ }
func (f *fakeWindowFrame) Maximize()         { f.maximized++; f.stillMaxed = true }
func (f *fakeWindowFrame) Unmaximize()       { f.restored++; f.stillMaxed = false }
func (f *fakeWindowFrame) Close()            { f.closed++ }
func (f *fakeWindowFrame) IsMaximized() bool { return f.stillMaxed }

// TestFramedWindowKeepsOneSetOfControls reads the frame options: the view's own
// buttons on Windows and Linux, with the system's taken away, and the system's
// traffic lights on macOS.
func TestFramedWindowKeepsOneSetOfControls(t *testing.T) {
	previous := controlsAtRight
	defer func() { controlsAtRight = previous }()

	controlsAtRight = true
	o := framedWindow(mygo.WindowOptions{})
	if o.TitleBarStyle != mygo.TitleBarHidden {
		t.Errorf("TitleBarStyle = %q, want %q", o.TitleBarStyle, mygo.TitleBarHidden)
	}
	if !o.HideWindowButtons {
		t.Error("the window kept the system's buttons, which would sit beside the drawn ones")
	}

	controlsAtRight = false
	o = framedWindow(mygo.WindowOptions{})
	if o.TitleBarStyle != mygo.TitleBarHiddenInset {
		t.Errorf("TitleBarStyle = %q, want %q", o.TitleBarStyle, mygo.TitleBarHiddenInset)
	}
	if o.HideWindowButtons {
		t.Error("the window hid its traffic lights, which are macOS's own")
	}
}

// TestTheWindowButtonsSitAtTheTopRight is the layout Windows and Linux get: the
// controls belong at the right end of the card's header, as Windows draws them,
// and do what their labels say.
func TestTheWindowButtonsSitAtTheTopRight(t *testing.T) {
	previous := controlsAtRight
	controlsAtRight = true
	defer func() { controlsAtRight = previous }()

	a := newTestApp(t)
	f := &fakeWindowFrame{}
	a.frameStub = f
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()

	for _, label := range []string{
		locales.WindowClose(), locales.WindowMinimize(), locales.WindowMaximize(),
	} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q is not on screen; texts = %v", label, tt.Texts())
		}
		if rect.X <= 1280/2 {
			t.Errorf("%q is at x=%v, not in the window's right corner", label, rect.X)
		}
		if center := rect.Y + rect.H/2; center < uitokens.BarMid-2 || center > uitokens.BarMid+2 {
			t.Errorf("%q is centered at y=%v, not on the top bar's line %v", label, center, uitokens.BarMid)
		}
		if right := rect.X + rect.W; right > 1280-uitokens.CardGap {
			t.Errorf("%q ends at x=%v, past the card's edge", label, right)
		}
	}

	min, _ := tt.Find(locales.WindowMinimize())
	tt.ClickAt(min.X+min.W/2, min.Y+min.H/2)
	tt.Frame()
	if f.minimized != 1 {
		t.Fatalf("the minimize button called Minimize %d times, want once", f.minimized)
	}

	max, _ := tt.Find(locales.WindowMaximize())
	tt.ClickAt(max.X+max.W/2, max.Y+max.H/2)
	tt.Frame()
	if f.maximized != 1 || !f.stillMaxed {
		t.Fatalf("the maximize button did not maximize: %+v", f)
	}
	if _, ok := tt.Find(locales.WindowRestore()); !ok {
		t.Fatalf("the restore button is not on screen after maximizing; texts = %v", tt.Texts())
	}

	close, _ := tt.Find(locales.WindowClose())
	tt.ClickAt(close.X+close.W/2, close.Y+close.H/2)
	tt.Frame()
	if f.closed != 1 {
		t.Fatalf("the close button called Close %d times, want once", f.closed)
	}

	// The controls stay above a dialog, where a window's controls belong.
	a.settingsOpen = true
	tt.Frame()
	close, ok := tt.Find(locales.WindowClose())
	if !ok {
		t.Fatalf("the close button is not on screen over Settings; texts = %v", tt.Texts())
	}
	tt.ClickAt(close.X+close.W/2, close.Y+close.H/2)
	tt.Frame()
	if f.closed != 2 {
		t.Fatalf("the close button over Settings called Close %d times, want twice", f.closed)
	}
}
