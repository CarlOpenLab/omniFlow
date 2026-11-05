package main

import (
	"strings"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/locales"
)

// importCurl opens a curl command pasted into the URL field as the request
// it describes. The parser keeps what it can and says what it had to leave
// out; a command it cannot read leaves the composer as it was.
func (a *app) importCurl(cmd string) {
	r, notes, err := httpx.ParseCurl(cmd)
	if err != nil {
		a.setNotice(locales.CurlImportFailed(err.Error()))
		return
	}
	a.open(r, "")
	a.mode = modeComposer
	if len(notes) > 0 {
		a.setNotice(locales.CurlImportedNotes(strings.Join(notes, ", ")))
		return
	}
	a.setNotice(locales.CurlImported())
}
