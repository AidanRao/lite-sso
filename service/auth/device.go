package auth

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DeviceCookieName = "device_id"
	DeviceTTL        = 365 * 24 * time.Hour
)

// EnsureDeviceID returns the browser device identifier and a value that must
// be written to the response when the browser did not have one yet.
func EnsureDeviceID(request *http.Request) (string, bool) {
	if value, ok := DeviceIDFromRequest(request); ok {
		return value, false
	}
	return newDeviceID(), true
}

// DeviceIDFromRequest reads an existing device cookie without creating a new
// device identity. Challenge-bound authentication must use this form.
func DeviceIDFromRequest(request *http.Request) (string, bool) {
	if request == nil {
		return "", false
	}
	cookie, err := request.Cookie(DeviceCookieName)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	if !strings.HasPrefix(value, "dev_") || len(value) > 64 {
		return "", false
	}
	return value, true
}

func newDeviceID() string {
	if id, err := uuid.NewRandom(); err == nil {
		return "dev_" + id.String()
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("dev_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("dev_%x", buf)
}

// WriteDeviceCookie writes the long-lived, non-authenticating device cookie.
func WriteDeviceCookie(w http.ResponseWriter, deviceID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     DeviceCookieName,
		Value:    deviceID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(DeviceTTL.Seconds()),
	})
}

func requestIP(request *http.Request) string {
	return requestIPWithProxyHeaders(request, false)
}

func requestIPWithProxyHeaders(request *http.Request, trustProxyHeaders bool) string {
	if request == nil {
		return ""
	}
	if trustProxyHeaders {
		for _, header := range []string{"X-Vercel-Forwarded-For", "X-Forwarded-For"} {
			if ip := firstValidForwardedIP(request.Header.Get(header)); ip != "" {
				return ip
			}
		}
	}
	host := request.RemoteAddr
	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			if parsedIP := net.ParseIP(parsedHost); parsedIP != nil {
				return parsedIP.String()
			}
			return strings.TrimSpace(parsedHost)
		}
	}
	host = strings.TrimSpace(host)
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		return parsedIP.String()
	}
	return host
}

func firstValidForwardedIP(value string) string {
	for _, candidate := range strings.Split(value, ",") {
		parsedIP := net.ParseIP(strings.TrimSpace(candidate))
		if parsedIP != nil {
			return parsedIP.String()
		}
	}
	return ""
}

// RequestIP returns the request source address, optionally trusting proxy headers.
func RequestIP(request *http.Request, trustProxyHeaders bool) string {
	return requestIPWithProxyHeaders(request, trustProxyHeaders)
}
