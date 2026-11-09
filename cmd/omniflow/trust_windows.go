//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemTrusts asks Windows' own Root stores whether this certificate is
// trusted. Both the current user's store and the machine's are checked: a
// certificate imported without administrator rights lives in the former.
func systemTrusts(path string) (trusted, known bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	hash, err := certSHA1(raw)
	if err != nil {
		return false, false
	}
	root, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return false, false
	}
	locations := []uint32{
		windows.CERT_SYSTEM_STORE_CURRENT_USER,
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE,
	}
	opened := false
	for _, location := range locations {
		store, err := windows.CertOpenStore(
			windows.CERT_STORE_PROV_SYSTEM,
			0,
			0,
			location,
			uintptr(unsafe.Pointer(root)),
		)
		if err != nil {
			continue
		}
		opened = true
		cert, err := windows.CertFindCertificateInStore(
			store,
			windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
			0,
			windows.CERT_FIND_SHA1_HASH,
			unsafe.Pointer(&hash[0]),
			nil,
		)
		if err == nil && cert != nil {
			_ = windows.CertFreeCertificateContext(cert)
			_ = windows.CertCloseStore(store, 0)
			return true, true
		}
		_ = windows.CertCloseStore(store, 0)
	}
	return false, opened
}
