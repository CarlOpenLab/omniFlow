package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/CarlOpenLab/omniflow/locales"
)

// trustState is what the system said of the Root CA, last it was asked.
type trustState int

const (
	trustUnknown trustState = iota
	trustChecking
	trustYes
	trustNo
)

// checkTrust asks the system again whether it trusts the Root CA, off the
// main thread, as the dialog opens and when asked to.
func (a *app) checkTrust() {
	if a.ca == nil || a.caTrust == trustChecking || !trustCheckSupported() {
		return
	}
	a.caTrust = trustChecking
	path := a.ca.CertPath()
	go func() {
		trusted, known := systemTrusts(path)
		state := trustUnknown
		switch {
		case known && trusted:
			state = trustYes
		case known:
			state = trustNo
		}
		a.update(func() {
			// A certificate installed while capture runs lets the HTTPS
			// proxy be taken over without a restart; one removed from under
			// it must give HTTPS back just as quickly.
			a.recordTrust(state)
		})
	}()
}

// recordTrust saves what the system said about the Root CA and brings the
// system proxy in line: HTTPS is only intercepted while the CA is trusted,
// whether the change came from this app or from outside it.
func (a *app) recordTrust(state trustState) {
	a.caTrust = state
	a.syncSystemProxy()
}

// trustCA installs the Root CA in the user's trust store, off the main
// thread, and asks the system what it made of it.
func (a *app) trustCA() {
	if a.ca == nil || a.caTrust == trustChecking {
		return
	}
	a.caTrust = trustChecking
	path := a.ca.CertPath()
	go func() {
		if err := installCA(path); err != nil {
			a.update(func() {
				a.caTrust = trustNo
				a.setNotice(locales.NoticeCATrustFailed(err.Error()))
			})
			return
		}
		trusted, known := systemTrusts(path)
		state := trustUnknown
		switch {
		case known && trusted:
			state = trustYes
		case known:
			state = trustNo
		}
		a.update(func() {
			a.recordTrust(state)
			if state == trustYes {
				a.setNotice(locales.NoticeCATrusted())
			}
		})
	}()
}

// untrustCA removes that trust and asks the system again.
func (a *app) untrustCA() {
	if a.ca == nil || a.caTrust == trustChecking {
		return
	}
	a.caTrust = trustChecking
	path := a.ca.CertPath()
	go func() {
		if err := uninstallCA(path); err != nil {
			a.update(func() {
				a.caTrust = trustYes
				a.setNotice(locales.NoticeCAUntrustFailed(err.Error()))
			})
			return
		}
		trusted, known := systemTrusts(path)
		state := trustUnknown
		switch {
		case known && trusted:
			state = trustYes
		case known:
			state = trustNo
		}
		a.update(func() {
			a.recordTrust(state)
			if state == trustNo {
				a.setNotice(locales.NoticeCAUntrusted())
			}
		})
	}()
}

// certSHA1 names the certificate by the hash the platform's tools use for
// a thumbprint.
func certSHA1(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("no certificate in %s", path)
	}
	sum := sha1.Sum(block.Bytes)
	return hex.EncodeToString(sum[:]), nil
}
