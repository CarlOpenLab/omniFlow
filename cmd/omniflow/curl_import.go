package main

import (
	"strings"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
)

// importCurl opens a curl command pasted into the URL field as the request
// it describes. The parser keeps what it can and says what it had to leave
// out; a command it cannot read leaves the composer as it was.
func (a *app) importCurl(cmd string) {
	r, notes, err := httpx.ParseCurl(cmd)
	if err != nil {
		a.setNotice("Could not import the cURL command: " + err.Error())
		return
	}
	a.open(r, "")
	a.mode = modeComposer
	if len(notes) > 0 {
		a.setNotice("Imported from cURL; left out: " + strings.Join(notes, ", ") + ".")
		return
	}
	a.setNotice("Imported from cURL")
}
