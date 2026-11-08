package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealPath shows path in the platform's file manager, choosing the file
// where it can.
func revealPath(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Run()
	case "windows":
		return exec.Command("explorer.exe", "/select,"+path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Run()
	}
}
