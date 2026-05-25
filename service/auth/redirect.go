package auth

import (
	"net/url"
	"strings"

	"sso-server/common"
)

const defaultLoginRedirect = "/profile"

func NormalizeLoginRedirect(raw string) (string, error) {
	redirect := strings.TrimSpace(raw)
	if redirect == "" {
		return defaultLoginRedirect, nil
	}

	parsed, err := url.Parse(redirect)
	if err != nil {
		return "", common.ErrInvalidRedirect
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return "", common.ErrInvalidRedirect
	}

	if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		return "", common.ErrInvalidRedirect
	}

	return redirect, nil
}
