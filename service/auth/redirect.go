package auth

import (
	"net/url"
	"strings"

	"sso-server/common"
)

const (
	defaultLoginRedirect = "/profile"
	emailSetupRoute      = "/setup-email"
	emailVerifyRoute     = "/profile/access/emails/verify"
)

// NormalizeLoginRedirect accepts only local navigation targets.
func NormalizeLoginRedirect(raw string) (string, error) {
	redirect := strings.TrimSpace(raw)
	if redirect == "" {
		return defaultLoginRedirect, nil
	}
	if strings.ContainsAny(redirect, "\\\r\n\t") {
		return "", common.ErrInvalidRedirect
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

func emailSetupRedirect(redirect string) string {
	parsed, _ := url.Parse(redirect)
	// Reauthentication must return directly to the setup or verification page.
	if strings.TrimRight(parsed.Path, "/") == emailSetupRoute || strings.TrimRight(parsed.Path, "/") == emailVerifyRoute {
		return redirect
	}
	return emailSetupRoute + "?" + url.Values{"redirect": {redirect}}.Encode()
}
