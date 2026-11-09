//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
)

func trustSupported() bool { return true }

// systemTrusts asks Windows whether the certificate is in the current user's
// Root store. certutil lists the whole store whatever hash it is given, so
// PowerShell names the one certificate instead. known is false when it could
// not be asked.
func systemTrusts(path string) (trusted, known bool) {
	hash, err := certSHA1(path)
	if err != nil {
		return false, false
	}
	script := fmt.Sprintf(
		`if (Get-ChildItem Cert:\CurrentUser\Root | Where-Object { $_.Thumbprint -eq '%s' }) { exit 0 } else { exit 1 }`,
		hash)
	err = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run()
	if err == nil {
		return true, true
	}
	var exit *exec.ExitError
	return false, errors.As(err, &exit)
}

// installCA adds the certificate to the current user's Root store, which
// browsers that use the Windows store trust. It needs no administrator.
func installCA(path string) error {
	return exec.Command("certutil", "-user", "-addstore", "Root", path).Run()
}

func uninstallCA(path string) error {
	hash, err := certSHA1(path)
	if err != nil {
		return err
	}
	return exec.Command("certutil", "-user", "-delstore", "Root", hash).Run()
}
