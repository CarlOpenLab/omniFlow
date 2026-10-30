//go:build darwin

package main

import (
	"errors"
	"os/exec"
)

// systemTrusts asks macOS whether it trusts the certificate at path as a
// root, as an app's TLS would, by the basic X.509 policy: it is read-only,
// and changes nothing in the keychains. known is false when it could not ask.
func systemTrusts(path string) (trusted, known bool) {
	err := exec.Command("security", "verify-cert", "-q", "-c", path, "-p", "basic").Run()
	if err == nil {
		return true, true
	}
	var exit *exec.ExitError
	return false, errors.As(err, &exit)
}
