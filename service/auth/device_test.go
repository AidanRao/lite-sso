package auth

import (
	"net/http"
	"testing"
)

func TestRequestIP_ProxyTrustBoundary(t *testing.T) {
	testCases := []struct {
		name              string
		remoteAddr        string
		vercelForwarded   string
		forwarded         string
		trustProxyHeaders bool
		expected          string
	}{
		{
			name:              "ignore forwarded headers by default",
			remoteAddr:        "10.0.0.8:8080",
			vercelForwarded:   "203.0.113.10",
			forwarded:         "203.0.113.11",
			trustProxyHeaders: false,
			expected:          "10.0.0.8",
		},
		{
			name:              "prefer vercel forwarded address",
			remoteAddr:        "10.0.0.8:8080",
			vercelForwarded:   "203.0.113.10",
			forwarded:         "203.0.113.11",
			trustProxyHeaders: true,
			expected:          "203.0.113.10",
		},
		{
			name:              "use first valid forwarded address",
			remoteAddr:        "10.0.0.8:8080",
			vercelForwarded:   "invalid",
			forwarded:         "invalid, 198.51.100.12, 10.0.0.8",
			trustProxyHeaders: true,
			expected:          "198.51.100.12",
		},
		{
			name:              "accept forwarded ipv6",
			remoteAddr:        "10.0.0.8:8080",
			vercelForwarded:   "2001:db8::10",
			trustProxyHeaders: true,
			expected:          "2001:db8::10",
		},
		{
			name:              "fall back to remote ipv6",
			remoteAddr:        "[2001:db8::20]:443",
			vercelForwarded:   "invalid",
			forwarded:         "also-invalid",
			trustProxyHeaders: true,
			expected:          "2001:db8::20",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			request.RemoteAddr = testCase.remoteAddr
			request.Header.Set("X-Vercel-Forwarded-For", testCase.vercelForwarded)
			request.Header.Set("X-Forwarded-For", testCase.forwarded)

			if actual := RequestIP(request, testCase.trustProxyHeaders); actual != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, actual)
			}
		})
	}
}
