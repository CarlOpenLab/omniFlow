//go:build linux

package main

import "os"

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
