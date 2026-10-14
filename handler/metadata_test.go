package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"sso-server/service/auth"
)

func Test_LoginMetadataFromRequest_SharedExtraction(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("User-Agent", " Chrome/130.0\x00 ")
	require.Equal(t, auth.LoginMetadata{DeviceID: "dev-browser", IP: "203.0.113.10", UserAgent: "Chrome/130.0"}, LoginMetadataFromRequest(req, "dev-browser", true))
	require.Equal(t, "10.0.0.1", LoginMetadataFromRequest(req, "dev-browser", false).IP)
	require.Equal(t, auth.LoginMetadata{DeviceID: "dev-browser"}, LoginMetadataFromRequest(nil, "dev-browser", true))
}
