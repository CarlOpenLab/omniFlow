package main

import "testing"

func TestCardRadiusFollowsTheWindowCorner(t *testing.T) {
	for _, tc := range []struct {
		goos   string
		mac    int
		radius float32
	}{
		{"darwin", 26, 20}, // Tahoe's 28px window less the 8px gap
		{"darwin", 15, 8},  // a 10px window less 8 would be 2; the floor holds
		{"darwin", 0, 20},  // an unreadable version stays with the current one
		{"windows", 0, 8},  // 8px window less 8 would be square
		{"linux", 0, 8},    // 12px window less 8 is 4; the floor holds
	} {
		if got := cardRadiusForVersion(tc.goos, tc.mac); got != tc.radius {
			t.Errorf("cardRadiusForVersion(%q, %d) = %v, want %v", tc.goos, tc.mac, got, tc.radius)
		}
	}
}
