package main

import (
	"github.com/CarlOpenLab/omniflow/internal/systemproxy"
	"github.com/CarlOpenLab/omniflow/locales"
)

// refreshSystemProxy reads the system's proxy settings without changing
// them. Ours is true only when the app saved the snapshot before enabling.
func (a *app) refreshSystemProxy() {
	st := systemproxy.Current(a.dataDir())
	a.sysProxySupported = st.Supported
	a.sysProxyErr = st.Err
	a.sysProxyOn = st.Enabled && st.Ours
	a.sysProxyOurs = st.Ours
}

// setSystemProxy turns the operating system's HTTP proxy on or off. On can
// start the capture proxy too, since a system proxy pointing at a stopped
// one would break every connection.
func (a *app) setSystemProxy(on bool) {
	if !a.sysProxySupported {
		a.setNotice(locales.NoticeSystemProxyFailed(a.sysProxyErr))
		return
	}
	if on {
		if !a.proxyRunning() {
			a.startProxy()
		}
		if !a.proxyRunning() {
			return
		}
		if err := systemproxy.Enable(a.dataDir(), "127.0.0.1:"+a.proxyPort); err != nil {
			a.setNotice(locales.NoticeSystemProxyFailed(err.Error()))
			a.refreshSystemProxy()
			return
		}
		a.sysProxyOn, a.sysProxyOurs = true, true
		a.setNotice(locales.NoticeSystemProxyOn(a.proxyPort))
		return
	}
	if err := systemproxy.Disable(a.dataDir()); err != nil {
		a.setNotice(locales.NoticeSystemProxyFailed(err.Error()))
		a.refreshSystemProxy()
		return
	}
	a.sysProxyOn, a.sysProxyOurs = false, false
	a.setNotice(locales.NoticeSystemProxyOff())
}

// restoreSystemProxy puts the system's proxy back as it was when OmniFlow
// quits, so a crash-free exit never leaves the machine pointed at a closed
// port.
func (a *app) restoreSystemProxy() {
	if !a.sysProxyOurs {
		return
	}
	_ = systemproxy.Disable(a.dataDir())
}
