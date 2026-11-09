//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

func trustSupported() bool { return true }

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

// installCA trusts the certificate for this user in the login keychain. The
// system may ask for the keychain's password.
func installCA(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	return exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-k", keychain, path).Run()
}

// uninstallCA removes the certificate's trust, and then the certificate
// itself, from the login keychain.
func uninstallCA(path string) error {
	hash, err := certSHA1(path)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	return exec.Command("security", "delete-certificate", "-Z", hash, "-t", keychain).Run()
}
