package sysproxy

import "fmt"

// wininetProxyServer is the WinINet ProxyServer value for a local proxy.
// WinINet accepts one address for every protocol, or a per-protocol list;
// naming only http leaves HTTPS direct.
func wininetProxyServer(port int, captureHTTPS bool) string {
	server := fmt.Sprintf("http=127.0.0.1:%d", port)
	if captureHTTPS {
		server += fmt.Sprintf(";https=127.0.0.1:%d", port)
	}
	return server
}
