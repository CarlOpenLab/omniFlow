//go:build !darwin && !windows && !linux

package main

// systemTrusts cannot ask away from macOS: whether the certificate is
// trusted is not known.
func systemTrusts(string) (trusted, known bool) { return false, false }
