package main

import (
	"runtime"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

// The workspace card's corner follows the window's, the way Beacon's does: the
// card sits CardGap inside a frame the system rounds, so its corner is the
// window's radius less that gap. The two arcs then share a centre and the frame
// reads as one even width all the way round. A number chosen on its own drifts
// from the system's as soon as macOS changes its rounding — 26 (Tahoe) made
// windows much rounder than the 10 every release before it used.

const (
	macTahoe = 26
	// minCardRadius keeps a card round where the arithmetic alone would make
	// it square: an 8px window less an 8px frame pinches the wash at the
	// corner instead of widening it.
	minCardRadius = 8
)

// windowRadius is the corner the system draws around the window. These are
// facts about the platform, not tokens of the app's own radius scale.
func windowRadius(goos string, macMajor int) float32 {
	switch goos {
	case "darwin":
		if macMajor > 0 && macMajor < macTahoe {
			return 10
		}
		return 28
	case "windows":
		return 8
	default:
		return 12
	}
}

// cardRadiusForVersion is the workspace card's corner for a platform at a
// macOS major version (0 when it has none).
func cardRadiusForVersion(goos string, macMajor int) float32 {
	return max(minCardRadius, windowRadius(goos, macMajor)-float32(uitokens.CardGap))
}

// cardRadius is the workspace card's corner on this machine.
func cardRadius() float32 {
	return cardRadiusForVersion(runtime.GOOS, macMajorVersion())
}
