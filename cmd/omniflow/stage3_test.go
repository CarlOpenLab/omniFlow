package main

import (
	"strings"
	"testing"

	"github.com/CarlOpenLab/omniflow/locales"
)

func TestCertSHA1ReadsTheRootCA(t *testing.T) {
	a := newTestApp(t)
	if a.ca == nil {
		t.Fatal("the test app has no CA")
	}
	hash, err := certSHA1(a.ca.CertPath())
	if err != nil {
		t.Fatalf("certSHA1: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("SHA-1 is %q, want 40 hex characters", hash)
	}
}

func TestDeviceSetupPageCarriesTheProxyAndCertificate(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = "9090"
	page := a.setupPage("/setup/token")
	for _, want := range []string{"/setup/token/ca.pem", "9090", "OmniFlow Root CA", "Manual"} {
		if !strings.Contains(page, want) {
			t.Errorf("the setup page does not mention %q", want)
		}
	}
}

func TestDeviceSetupDialogRenders(t *testing.T) {
	a := newTestApp(t)
	a.deviceSetupOpen = true
	tt := newTester(t, a)
	tt.Frame()
	if !tt.HasText(locales.DeviceSetupTitle()) || !tt.HasText(locales.SettingsLANAccess()) {
		t.Fatalf("the device setup dialog is missing; texts = %v", tt.Texts())
	}

	a.lanAccess = true
	a.proxyPort = "9090"
	a.setupURL = "http://192.168.1.2:9091/setup/token"
	tt.Frame()
	for _, want := range []string{locales.DeviceSetupProxy(), locales.DeviceSetupPage()} {
		if !tt.HasText(want) {
			t.Errorf("the LAN setup dialog is missing %q", want)
		}
	}
}

func TestCADialogOffersTrustWhenSupported(t *testing.T) {
	a := newTestApp(t)
	a.caOpen, a.caWasOpen = true, true
	a.caTrust = trustNo
	tt := newTester(t, a)
	tt.Frame()
	if trustSupported() {
		if !tt.HasText(locales.CaTrust()) {
			t.Fatalf("the CA dialog has no Trust button; texts = %v", tt.Texts())
		}
	} else if !tt.HasText(locales.CaTrustUnavailable()) {
		t.Fatalf("the CA dialog does not say trust is unavailable; texts = %v", tt.Texts())
	}

	a.caTrust = trustYes
	tt.Frame()
	if trustSupported() && !tt.HasText(locales.CaUntrust()) {
		t.Errorf("the trusted CA dialog has no Remove Trust button; texts = %v", tt.Texts())
	}
}

func TestSystemTrustsSaysAFreshCAIsNotTrusted(t *testing.T) {
	if !trustSupported() {
		t.Skip("this platform has no one-click trust")
	}
	a := newTestApp(t)
	trusted, known := systemTrusts(a.ca.CertPath())
	if !known {
		t.Fatal("the system could not be asked about the Root CA")
	}
	if trusted {
		t.Fatal("a certificate that was never installed is reported trusted")
	}
}
