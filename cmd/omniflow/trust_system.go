package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

// certSHA1 is the certificate's DER SHA-1 fingerprint, the form Windows
// certificate stores search by.
func certSHA1(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a certificate")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, err
	}
	sum := sha1.Sum(block.Bytes)
	return sum[:], nil
}

// bundlesContainCert reports whether one of the PEM bundles holds this
// certificate. known is false when none of the bundle paths could be read,
// so a caller can tell "not trusted" from "cannot tell".
func bundlesContainCert(certPEM []byte, paths []string) (trusted, known bool) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, false
	}
	target := block.Bytes
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		known = true
		for len(raw) > 0 {
			var cert *pem.Block
			cert, raw = pem.Decode(raw)
			if cert == nil {
				break
			}
			if cert.Type == "CERTIFICATE" && bytes.Equal(cert.Bytes, target) {
				return true, true
			}
		}
	}
	return false, known
}
