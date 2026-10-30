package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CA is OmniFlow's local certificate authority: the one certificate a
// person installs to let the inspector read HTTPS, and the signer of the
// certificates it presents for the hosts passing through.
type CA struct {
	certPath string
	keyPath  string
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
	certPEM  []byte

	mu    sync.Mutex
	leafs map[string]*tls.Certificate
}

// LoadOrCreateCA loads the authority in dir, or creates it there the first
// time. The certificate is public; the key stays 0600 and never leaves the
// app's own directory unless the person exports it.
func LoadOrCreateCA(dir string) (*CA, error) {
	ca := &CA{
		certPath: filepath.Join(dir, "omniflow-ca.pem"),
		keyPath:  filepath.Join(dir, "omniflow-ca-key.pem"),
		leafs:    map[string]*tls.Certificate{},
	}
	certPEM, certErr := os.ReadFile(ca.certPath)
	keyPEM, keyErr := os.ReadFile(ca.keyPath)
	if certErr == nil && keyErr == nil {
		cert, err := parseCertificate(certPEM)
		if err == nil {
			key, err := parseKey(keyPEM)
			if err == nil {
				ca.cert, ca.key, ca.certPEM = cert, key, certPEM
				return ca, nil
			}
		}
		// A damaged pair is replaced: the old files stay beside the new ones
		// so an installed copy can be compared, but a proxy that cannot sign
		// is not worth keeping.
		_ = os.Rename(ca.certPath, ca.certPath+".bad")
		_ = os.Rename(ca.keyPath, ca.keyPath+".bad")
	}
	if err := ca.create(dir); err != nil {
		return nil, err
	}
	return ca, nil
}

func (ca *CA) create(dir string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"OmniFlow"},
			CommonName:   "OmniFlow Root CA",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ca.certPath, certPEM, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(ca.keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	ca.cert, ca.key, ca.certPEM = cert, key, certPEM
	return nil
}

// CertPEM is the public certificate, the file a person installs in the
// system's trust store.
func (ca *CA) CertPEM() []byte { return ca.certPEM }

// CertPath is where that file lives.
func (ca *CA) CertPath() string { return ca.certPath }

// Leaf returns a certificate for host, signed by the authority and cached
// for the life of the proxy. A host that is an IP address gets an IP SAN
// rather than a DNS one.
func (ca *CA) Leaf(host string) (*tls.Certificate, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	host = strings.Trim(host, "[]")
	ca.mu.Lock()
	if c, ok := ca.leafs[host]; ok {
		ca.mu.Unlock()
		return c, nil
	}
	ca.mu.Unlock()

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"OmniFlow"}, CommonName: host},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(397 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}
	cert := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	ca.mu.Lock()
	ca.leafs[host] = cert
	ca.mu.Unlock()
	return cert, nil
}

func parseCertificate(raw []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseKey(raw []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("not a key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is %T, not ECDSA", key)
	}
	return ec, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
