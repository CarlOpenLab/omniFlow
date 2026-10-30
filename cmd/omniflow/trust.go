package main

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
	if a.ca == nil || a.caTrust == trustChecking {
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
		a.update(func() { a.caTrust = state })
	}()
}
