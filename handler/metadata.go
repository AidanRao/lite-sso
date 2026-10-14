// Package handler adapts HTTP request data for the application services.
package handler

import (
	"net/http"

	"sso-server/service/auth"
	"sso-server/util/requestmeta"
)

// LoginMetadataFromRequest combines the browser cookie identity with request diagnostics.
func LoginMetadataFromRequest(request *http.Request, deviceID string, trustProxyHeaders bool) auth.LoginMetadata {
	metadata := requestmeta.Extract(request, trustProxyHeaders)
	return auth.LoginMetadata{DeviceID: deviceID, IP: metadata.IP, UserAgent: metadata.UserAgent}
}
