//go:build !darwin && !windows && !linux

package main

import "errors"

func trustSupported() bool { return false }

// systemTrusts cannot ask away from macOS and Windows: whether the
// certificate is trusted is not known.
func systemTrusts(string) (trusted, known bool) { return false, false }

func installCA(string) error {
	return errors.New("install the certificate in the system's trust store manually")
}

func uninstallCA(string) error {
	return errors.New("remove the certificate in the system's trust store manually")
}
