package oauth2

import (
	"encoding/json"
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

	for _, u := range allowed {
		if strings.TrimSpace(u) == requested {
			return requested, nil
		}
	}
	return "", oauth2errors.ErrInvalidRedirectURI
}
