package oauth2

import (
	"encoding/json"
	"net/url"
	"strings"

	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
)

func ValidateRedirectURI(baseURI string, redirectURI string) error {
	baseURI = strings.TrimSpace(baseURI)
	redirectURI = strings.TrimSpace(redirectURI)

	if baseURI == "" || redirectURI == "" {
		return oauth2errors.ErrInvalidRedirectURI
	}

	redirectURL, err := url.Parse(redirectURI)
	if err != nil || redirectURL.Host == "" {
		return oauth2errors.ErrInvalidRedirectURI
	}
	redirectHost := redirectURL.Host

	if strings.HasPrefix(baseURI, "[") {
		var allowed []string
		if err := json.Unmarshal([]byte(baseURI), &allowed); err != nil {
			return oauth2errors.ErrInvalidRedirectURI
		}
		for _, u := range allowed {
			allowedURL, err := url.Parse(strings.TrimSpace(u))
			if err != nil {
				continue
			}
			if allowedURL.Host == redirectHost {
				return nil
			}
		}
		return oauth2errors.ErrInvalidRedirectURI
	}

	allowedURL, err := url.Parse(baseURI)
	if err != nil {
		return oauth2errors.ErrInvalidRedirectURI
	}
	if allowedURL.Host != redirectHost {
		return oauth2errors.ErrInvalidRedirectURI
	}
	return nil
}
