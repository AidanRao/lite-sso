package oauth

import (
	"sso-server/manager/thirdparty"
)

type thirdPartyProvider = thirdparty.Provider
type thirdPartyProfile = thirdparty.Profile

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
