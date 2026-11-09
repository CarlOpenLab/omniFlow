package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCertPEM(t *testing.T, commonName string) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), der
}

func TestCertSHA1(t *testing.T) {
	pemBytes, der := testCertPEM(t, "OmniFlow Test CA")
	got, err := certSHA1Bytes(pemBytes)
	if err != nil {
		t.Fatalf("certSHA1Bytes: %v", err)
	}
	want := sha1.Sum(der)
	if string(got) != string(want[:]) {
		t.Fatalf("certSHA1Bytes = %x, want %x", got, want)
	}

	if _, err := certSHA1Bytes([]byte("not a certificate")); err == nil {
		t.Fatal("certSHA1Bytes accepted non-PEM input")
	}
}

func TestBundlesContainCert(t *testing.T) {
	ourPEM, _ := testCertPEM(t, "OmniFlow Test CA")
	otherPEM, _ := testCertPEM(t, "Some Other CA")
	dir := t.TempDir()

	with := filepath.Join(dir, "with.pem")
	if err := os.WriteFile(with, append(append([]byte{}, otherPEM...), ourPEM...), 0o644); err != nil {
		t.Fatal(err)
	}
	without := filepath.Join(dir, "without.pem")
	if err := os.WriteFile(without, otherPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	if trusted, known := bundlesContainCert(ourPEM, []string{without, with}); !trusted || !known {
		t.Fatalf("with the certificate in a bundle: trusted=%v known=%v, want true and true", trusted, known)
	}
	if trusted, known := bundlesContainCert(ourPEM, []string{without}); trusted || !known {
		t.Fatalf("without the certificate: trusted=%v known=%v, want false and true", trusted, known)
	}
	if trusted, known := bundlesContainCert(ourPEM, []string{filepath.Join(dir, "missing.pem")}); trusted || known {
		t.Fatalf("with no readable bundle: trusted=%v known=%v, want false and false", trusted, known)
	}
}
