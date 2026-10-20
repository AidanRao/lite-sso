package oauth2

import (
	"encoding/json"
	"strings"

	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
)

func ValidateRedirectURI(baseURI string, redirectURI string) error {
	baseURI = strings.TrimSpace(baseURI)

	if baseURI == "" || redirectURI == "" {
		return oauth2errors.ErrInvalidRedirectURI
	}
	if strings.HasPrefix(baseURI, "[") {
		var allowed []string
		if err := json.Unmarshal([]byte(baseURI), &allowed); err != nil {
			return oauth2errors.ErrInvalidRedirectURI
		}
		for _, value := range allowed {
			if value == redirectURI {
				return nil
			}
		}
		return oauth2errors.ErrInvalidRedirectURI
	}
	if baseURI != redirectURI {
		return oauth2errors.ErrInvalidRedirectURI
	}
	return nil
}
