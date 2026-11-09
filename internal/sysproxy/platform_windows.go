//go:build windows

package sysproxy

// defaultOps uses the per-user WinINet settings, which need no administrator
// rights and are what browsers on Windows follow.
func defaultOps() platformOps { return newWindowsOps() }
