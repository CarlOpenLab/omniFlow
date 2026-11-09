//go:build darwin

package sysproxy

// defaultOps returns the macOS implementation, filled in by darwin.go.
func defaultOps() platformOps { return unsupportedOps{name: "darwin"} }
