//go:build linux

package main

import (
	"errors"
	"os"
)

// trustSupported is false: reading the bundle is enough for capture to
// decide whether to proxy HTTPS, but adding to it needs a package manager
// or administrator tools this app does not drive.
func trustSupported() bool { return false }

// trustCheckSupported is true: the bundle can be read even though this app
// cannot write to it, so the dialog still shows the live trust state.
func trustCheckSupported() bool { return true }

func installCA(string) error {
	return errors.New("install the certificate in the system's trust store manually")
}

func uninstallCA(string) error {
	return errors.New("remove the certificate from the system's trust store manually")
}

// linuxTrustBundles are the usual system CA files on Debian and RHEL
// families. A certificate installed only in a browser's private store does
// not count; the system proxy then stays off for HTTPS.
var linuxTrustBundles = []string{
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/ca-bundle.pem",
	"/etc/ssl/cert.pem",
}

func systemTrusts(path string) (trusted, known bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	return bundlesContainCert(raw, linuxTrustBundles)
}
