//go:build linux

package main

import "testing"

// Linux cannot install the Root CA for the user, but it can read the
// system's bundles: the dialog must still show the live trust state and
// offer Check Again.
func TestLinuxCanCheckTrustWithoutInstallingIt(t *testing.T) {
	if trustSupported() {
		t.Fatal("linux claims one-click trust it cannot do")
	}
	if !trustCheckSupported() {
		t.Fatal("linux cannot ask whether the Root CA is trusted")
	}
}
