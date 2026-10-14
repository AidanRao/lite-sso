package requestmeta

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Extract_ProxyTrustAndNormalization(t *testing.T) {
	cases := []struct {
		name, remote, vercel, forwarded, expected string
		trust                                     bool
	}{
		{name: "ignore untrusted headers", remote: "10.0.0.8:8080", vercel: "203.0.113.10", forwarded: "203.0.113.11", expected: "10.0.0.8"},
		{name: "prefer vercel", remote: "10.0.0.8:8080", vercel: "203.0.113.10", forwarded: "203.0.113.11", trust: true, expected: "203.0.113.10"},
		{name: "first valid forwarded", remote: "10.0.0.8:8080", vercel: "invalid", forwarded: "invalid, 198.51.100.12, 10.0.0.8", trust: true, expected: "198.51.100.12"},
		{name: "forwarded ipv6", remote: "10.0.0.8:8080", vercel: "2001:0db8::10", trust: true, expected: "2001:db8::10"},
		{name: "remote ipv6 fallback", remote: "[2001:db8::20]:443", forwarded: "invalid", trust: true, expected: "2001:db8::20"},
		{name: "invalid remote is not an IP", remote: "private-host:443", expected: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.test", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("X-Vercel-Forwarded-For", tc.vercel)
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			req.Header.Set("User-Agent", " Chrome/130.0\x00 ")
			metadata := Extract(req, tc.trust)
			require.Equal(t, tc.expected, metadata.IP)
			require.Equal(t, "Chrome/130.0", metadata.UserAgent)
		})
	}
	require.Equal(t, Metadata{}, Extract(nil, true))
}

func Test_ParseEnvironment_NormalizesStoredMetadata(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
	environment := ParseEnvironment("2001:0db8::1", ua, "")
	require.Equal(t, "Chrome · macOS", environment.DisplayName)
	require.Equal(t, "Mac", environment.Label)
	require.Equal(t, "10.15.7", environment.OSVersion)
	require.Equal(t, "2001:db8::1", environment.IP)
	unknown := ParseEnvironment("private-host", "private-client-secret/1.0", "")
	require.Equal(t, "未知设备", unknown.DisplayName)
	require.Empty(t, unknown.IP)
	require.Empty(t, unknown.Browser)
	require.Empty(t, unknown.Label)
}
