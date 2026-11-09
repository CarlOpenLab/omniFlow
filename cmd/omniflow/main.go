// Command omniflow is OmniFlow's desktop app: a native window drawn with
// Mygo, holding an HTTP composer and a live capture proxy in one workbench.
//
// There is no webview: every pixel is drawn by Mygo's GPU canvas, and the
// window's acrylic is the system's own material showing through the view.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

func main() {
	// The name decides the user-data directory, where the collection, the
	// history and the capture certificate live.
	mygo.App.SetName("OmniFlow")

	// Auto-update: the app checks its own release feed and offers new
	// versions with their release notes. The native plugin draws that window
	// with mygo's own UI rather than a web page, as the rest of this app is
	// drawn. Builds never signed for updates — a development build, or a .deb
	// a package manager installed — say so instead of checking, and Settings
	// shows that reason rather than switches that would do nothing.
	mygo.Use(native.New(updater.Options{}))

	a := newApp()
	a.applyPreferences()

	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(a.windowOptions(ui.View(a.view)))
		a.win.OnClosed(func() {
			a.persistOnClose()
			a.stopSetupServer()
			a.restoreSystemProxy()
			if a.prx != nil {
				a.prx.Stop()
			}
			mygo.App.Quit()
		})
		a.installMenu()
		// The app's own menu labels are built here, not by MyGo, so they
		// are made again in the new language when the app shows another.
		mygo.App.OnLocaleChanged(func(string) { a.installMenu() })
		if a.lanAccess {
			if err := a.ensureSetupServer(); err != nil {
				a.setupErr = err.Error()
			}
		}
		if a.prefs.ProxyAutostart {
			a.startProxy()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	if a.prx != nil {
		a.prx.Stop()
	}
}
