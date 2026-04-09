package oauth2

import (
	"encoding/json"
	"net/url"
	"strings"

	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
)

func ResolveRedirectURI(redirectURIs string, requested string) (string, error) {
	redirectURIs = strings.TrimSpace(redirectURIs)
	requested = strings.TrimSpace(requested)

	if redirectURIs == "" {
		return "", oauth2errors.ErrInvalidRedirectURI
	}

	var allowed []string
	if err := json.Unmarshal([]byte(redirectURIs), &allowed); err != nil {
		return "", oauth2errors.ErrInvalidRedirectURI
	}
	if len(allowed) == 0 {
		return "", oauth2errors.ErrInvalidRedirectURI
	}

	if requested == "" {
		return strings.TrimSpace(allowed[0]), nil
	}

	requestedURL, err := url.Parse(requested)
	if err != nil || requestedURL.Host == "" {
		return "", oauth2errors.ErrInvalidRedirectURI
	}
	requestedHost := requestedURL.Host

	for _, u := range allowed {
		allowedURL, err := url.Parse(strings.TrimSpace(u))
		if err != nil {
			continue
		}
		if allowedURL.Host == requestedHost {
			return requested, nil
		}
	}
	return "", oauth2errors.ErrInvalidRedirectURI
}
