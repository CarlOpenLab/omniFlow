package main

import (
	"time"

	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/locales"
)

// The simulated networks the Inspector offers. The profile is stored in the
// preferences, so it survives a restart.
const (
	throttleOff    = ""
	throttleSlow3G = "slow3g"
	throttleFast3G = "fast3g"
	throttle4G     = "4g"
)

func normalizeThrottle(profile string) string {
	switch profile {
	case throttleSlow3G, throttleFast3G, throttle4G:
		return profile
	}
	return throttleOff
}

func throttleLabels() []string {
	return []string{
		locales.ThrottleOff(),
		locales.ThrottleSlow3G(),
		locales.ThrottleFast3G(),
		locales.Throttle4G(),
	}
}

func throttleLabel(profile string) string {
	switch normalizeThrottle(profile) {
	case throttleSlow3G:
		return locales.ThrottleSlow3G()
	case throttleFast3G:
		return locales.ThrottleFast3G()
	case throttle4G:
		return locales.Throttle4G()
	}
	return locales.ThrottleOff()
}

func throttleFromLabel(label string) string {
	switch label {
	case locales.ThrottleSlow3G():
		return throttleSlow3G
	case locales.ThrottleFast3G():
		return throttleFast3G
	case locales.Throttle4G():
		return throttle4G
	}
	return throttleOff
}

func throttleSpec(profile string) proxy.Throttle {
	switch normalizeThrottle(profile) {
	case throttleSlow3G:
		return proxy.Throttle{Latency: 400 * time.Millisecond, BytesPerSec: 400 << 10}
	case throttleFast3G:
		return proxy.Throttle{Latency: 150 * time.Millisecond, BytesPerSec: 1600 << 10}
	case throttle4G:
		return proxy.Throttle{Latency: 70 * time.Millisecond, BytesPerSec: 4 << 20}
	}
	return proxy.Throttle{}
}

func (a *app) applyThrottle() {
	if a.prx != nil {
		a.prx.SetThrottle(throttleSpec(a.throttle))
	}
}

func (a *app) setThrottle(profile string) {
	profile = normalizeThrottle(profile)
	if profile == a.throttle && profile == a.prefs.Throttle {
		return
	}
	a.throttle = profile
	a.prefs.Throttle = profile
	a.savePrefs()
	a.applyThrottle()
}
