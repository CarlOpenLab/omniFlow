package sysproxy

import "testing"

func TestWininetProxyServer(t *testing.T) {
	cases := []struct {
		name   string
		port   int
		https  bool
		expect string
	}{
		{"http only", 9090, false, "http=127.0.0.1:9090"},
		{"http and https", 9090, true, "http=127.0.0.1:9090;https=127.0.0.1:9090"},
		{"another port", 8080, true, "http=127.0.0.1:8080;https=127.0.0.1:8080"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wininetProxyServer(c.port, c.https); got != c.expect {
				t.Fatalf("wininetProxyServer(%d, %v) = %q, want %q", c.port, c.https, got, c.expect)
			}
		})
	}
}
