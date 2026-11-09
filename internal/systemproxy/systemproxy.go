// Package systemproxy changes the operating system's HTTP proxy and puts it
// back. It is supported on Windows; elsewhere Current reports it as
// unsupported and the app tells the person to use the system settings.
package systemproxy

// Status is what the system's HTTP proxy says now.
type Status struct {
	Supported bool
	Enabled   bool
	// Ours is true when the saved snapshot says this app set the proxy.
	Ours   bool
	Server string
	Err    string
}
