package oauth2

import (
	"strings"

	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
)

func ResolveRedirectURI(redirectURI string, requested string) (string, error) {
	redirectURI = strings.TrimSpace(redirectURI)
	requested = strings.TrimSpace(requested)

	if redirectURI == "" || requested == "" {
		return "", oauth2errors.ErrInvalidRedirectURI
	}
	if redirectURI != requested {
		return "", oauth2errors.ErrInvalidRedirectURI
	}
	return requested, nil
}
