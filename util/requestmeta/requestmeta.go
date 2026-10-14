// Package requestmeta extracts bounded request metadata with an explicit proxy trust policy.
package requestmeta

import (
	"net"
	"net/http"
	"strings"

	"sso-server/util/useragent"
)

// Metadata holds diagnostic input; API responses must use a display DTO instead.
type Metadata struct {
	IP        string
	UserAgent string
}

// Environment contains parsed display information without the diagnostic User-Agent.
type Environment struct {
	useragent.DeviceInfo
	IP       string
	Location string
}

// Extract reads only the source IP and User-Agent, never cookies or credentials.
func Extract(request *http.Request, trustProxyHeaders bool) Metadata {
	if request == nil {
		return Metadata{}
	}
	return Metadata{IP: IP(request, trustProxyHeaders), UserAgent: useragent.Normalize(request.UserAgent())}
}

// IP uses forwarding headers only when the deployment explicitly trusts its ingress.
// The ingress must replace untrusted client headers before forwarding requests.
func IP(request *http.Request, trustProxyHeaders bool) string {
	if request == nil {
		return ""
	}
	if trustProxyHeaders {
		for _, header := range []string{"X-Vercel-Forwarded-For", "X-Forwarded-For"} {
			for _, candidate := range strings.Split(request.Header.Get(header), ",") {
				if ip := NormalizeIP(candidate); ip != "" {
					return ip
				}
			}
		}
	}
	host := strings.TrimSpace(request.RemoteAddr)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return NormalizeIP(host)
}

// NormalizeIP returns a canonical IP address, or an empty value for invalid input.
func NormalizeIP(value string) string {
	if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
		return ip.String()
	}
	return ""
}

// ParseEnvironment derives display information from current or stored request metadata.
func ParseEnvironment(ip, rawUserAgent, location string) Environment {
	return Environment{DeviceInfo: useragent.Parse(rawUserAgent), IP: NormalizeIP(ip), Location: location}
}
