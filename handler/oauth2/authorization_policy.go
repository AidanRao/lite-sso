package oauth2

import (
	"net/http"

	gooauth2 "github.com/go-oauth2/oauth2/v4"
	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
)

// AuthorizationCodePolicy applies per-client PKCE requirements to authorization requests.
type AuthorizationCodePolicy struct{}

// ValidateAuthorizationRequest validates PKCE parameters for the client type.
func (AuthorizationCodePolicy) ValidateAuthorizationRequest(request *http.Request, isPublic bool) error {
	challenge := request.FormValue("code_challenge")
	method := request.FormValue("code_challenge_method")

	if challenge == "" {
		if isPublic || method != "" {
			return oauth2errors.ErrCodeChallengeRquired
		}
		return nil
	}
	if method != string(gooauth2.CodeChallengeS256) {
		return oauth2errors.ErrUnsupportedCodeChallengeMethod
	}
	return nil
}
