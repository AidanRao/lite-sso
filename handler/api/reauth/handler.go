// Package reauth exposes authenticated re-authentication APIs.
package reauth

import (
	"sso-server/conf"
	servicepasskey "sso-server/service/passkey"
	servicereauth "sso-server/service/reauth"
)

// Deps contains re-authentication handler dependencies.
type Deps struct {
	Config  *conf.Config
	Passkey *servicepasskey.Service
	Reauth  *servicereauth.Service
}

// Handler handles all re-authentication methods.
type Handler struct {
	passkey           *servicepasskey.Service
	reauth            *servicereauth.Service
	trustProxyHeaders bool
}

// NewHandler creates a re-authentication handler.
func NewHandler(deps Deps) *Handler {
	return &Handler{
		passkey:           deps.Passkey,
		reauth:            deps.Reauth,
		trustProxyHeaders: deps.Config != nil && deps.Config.Server.TrustProxyHeaders,
	}
}
