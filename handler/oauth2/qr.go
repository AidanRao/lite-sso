package oauth2

import (
	"net/http"
	"slices"
	"strings"

	"sso-server/common"
	"sso-server/service/auth"
)

// ValidateQRCodeToken verifies an OAuth token without accepting first-party sessions.
// Permission presence is separate so callers can distinguish stale tokens from
// applications whose administrator has revoked QR login permission.
func (o *OAuth2) ValidateQRCodeToken(request *http.Request) (auth.QRActor, bool, error) {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if len(header) < 8 || !strings.EqualFold(header[:7], "Bearer ") {
		return auth.QRActor{}, false, common.ErrInvalidCredentials
	}
	claims, err := o.signer.parse(strings.TrimSpace(header[7:]))
	if err != nil {
		return auth.QRActor{}, false, common.ErrInvalidCredentials
	}
	permitted := slices.Contains(claims.Audience, auth.QRLoginAudience) && slices.Contains(strings.Fields(claims.Scope), auth.QRLoginScope)
	return auth.QRActor{UserID: claims.Subject, ClientID: claims.ClientID}, permitted, nil
}
