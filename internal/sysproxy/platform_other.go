//go:build !darwin && !windows

package sysproxy

// defaultOps is replaced by the platform files as each system is supported.
func defaultOps() platformOps { return unsupportedOps{} }
