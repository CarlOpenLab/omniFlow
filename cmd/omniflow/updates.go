package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"github.com/CarlOpenLab/omniflow/locales"
)

// updatePrefs is what the Updates page shows: whether this build can update
// itself at all, the two switches, and when it last asked.
type updatePrefs struct {
	available          bool
	automaticChecks    bool
	automaticDownloads bool
	lastCheck          time.Time
}

// The plugin is registered by main, and asking it anything needs the app's
// event loop. A test process has neither, so the three calls go through
// variables; tests replace them, the window uses the real ones.
var (
	loadUpdatePrefs = func() updatePrefs {
		// Enabled is false for development builds and for apps a package
		// manager installed, both of which the plugin refuses to update.
		if !mygo.Updater.Enabled() {
			return updatePrefs{}
		}
		return updatePrefs{
			available:          true,
			automaticChecks:    updater.AutomaticChecks(),
			automaticDownloads: updater.AutomaticDownloads(),
			lastCheck:          updater.LastCheck(),
		}
	}
	saveUpdatePrefs = func(automaticChecks, automaticDownloads bool) {
		updater.SetAutomaticChecks(automaticChecks)
		updater.SetAutomaticDownloads(automaticDownloads)
	}
	checkForUpdatesNow = func() { updater.CheckForUpdates() }
)

// updaterMenuItem is the Check for Updates… item. It is a variable rather
// than a call so a test can replace it: updater.MenuItem asks the plugin,
// which needs the app's event loop.
var updaterMenuItem = func() *mygo.MenuItem { return updater.MenuItem() }

// lastCheckLabel says when the app last asked about updates.
func lastCheckLabel(when time.Time) string {
	if when.IsZero() {
		return locales.UpdateNever()
	}
	return when.Local().Format("2006-01-02 15:04")
}
