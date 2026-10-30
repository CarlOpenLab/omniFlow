//go:build !darwin

package main

// macMajorVersion has no meaning away from macOS; the window radius falls back
// to the platform's own corner.
func macMajorVersion() int { return 0 }
