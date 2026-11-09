//go:build darwin

package sysproxy

// defaultOps uses networksetup, which any administrator on this Mac can run
// without a helper or an authentication prompt.
func defaultOps() platformOps { return newDarwinOps(execRunner{}) }
