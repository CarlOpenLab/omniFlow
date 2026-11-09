//go:build linux

package sysproxy

// defaultOps uses gsettings, which per-user GNOME desktops let an ordinary
// session change without root.
func defaultOps() platformOps { return newGsettingsOps(execRunner{}) }
