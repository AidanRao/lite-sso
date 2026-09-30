package oauth2

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2 "github.com/go-oauth2/oauth2/v4"
	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
	"github.com/golang-jwt/jwt/v5"

	"sso-server/conf"
	"sso-server/dal/db"
)

type accessClaims struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

type tokenSigner struct {
	issuer     string
	kid        string
	privateKey *rsa.PrivateKey
	publicKeys map[string]*rsa.PublicKey
	ttl        time.Duration
}

func newTokenSigner(cfg conf.OAuthTokenConfig) (*tokenSigner, error) {
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" || cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		return nil, errors.New("OAuth issuer and positive token lifetimes are required")
	}
	parsedIssuer, err := url.Parse(issuer)
	if err != nil || (parsedIssuer.Scheme != "https" && parsedIssuer.Scheme != "http") || parsedIssuer.Host == "" || parsedIssuer.RawQuery != "" || parsedIssuer.Fragment != "" {
		return nil, errors.New("tokens.oauth.issuer must be an HTTP URL without query or fragment")
	}
	kid := strings.TrimSpace(cfg.SigningKID)
	if kid == "" {
		return nil, errors.New("tokens.oauth.signing_kid is required")
	}
	privatePEM := strings.ReplaceAll(cfg.SigningPrivateKeyPEM, `\n`, "\n")
	if strings.TrimSpace(privatePEM) == "" {
		return nil, errors.New("tokens.oauth.signing_private_key_pem is required")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privatePEM))
	if err != nil {
		return nil, fmt.Errorf("parse OAuth signing key: %w", err)
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("OAuth RSA key must be at least 2048 bits")
	}
	keys := map[string]*rsa.PublicKey{kid: &key.PublicKey}
	if cfg.PreviousPublicKeys != "" {
		var previous map[string]string
		if err := json.Unmarshal([]byte(cfg.PreviousPublicKeys), &previous); err != nil {
			return nil, fmt.Errorf("parse previous OAuth public keys: %w", err)
		}
		for id, pem := range previous {
			if id == "" || id == kid {
				return nil, errors.New("duplicate or empty OAuth key id")
			}
			publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(strings.ReplaceAll(pem, `\n`, "\n")))
			if err != nil {
				return nil, fmt.Errorf("parse OAuth public key %s: %w", id, err)
			}
			keys[id] = publicKey
		}
	}
	return &tokenSigner{issuer: issuer, kid: kid, privateKey: key, publicKeys: keys, ttl: cfg.AccessTokenTTL}, nil
}

func (s *tokenSigner) sign(userID, clientID string, audiences, scopes []string, now time.Time) (string, error) {
	if userID == "" || clientID == "" || len(audiences) == 0 {
		return "", errors.New("OAuth token requires subject, client and audience")
	}
	claims := accessClaims{ClientID: clientID, Scope: strings.Join(scopes, " "), RegisteredClaims: jwt.RegisteredClaims{
		Issuer: s.issuer, Subject: userID, Audience: jwt.ClaimStrings(audiences),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.kid
	return token.SignedString(s.privateKey)
}

func (s *tokenSigner) parse(raw string) (*accessClaims, error) {
	claims := new(accessClaims)
	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		key := s.publicKeys[kid]
		if key == nil {
			return nil, errors.New("unknown OAuth key id")
		}
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}), jwt.WithIssuer(s.issuer), jwt.WithExpirationRequired())
	if err != nil || claims.Subject == "" || claims.ClientID == "" || len(claims.Audience) == 0 {
		return nil, errors.New("invalid OAuth access token")
	}
	return claims, nil
}

func (s *tokenSigner) jwks() map[string]interface{} {
	keys := make([]map[string]string, 0, len(s.publicKeys))
	for kid, key := range s.publicKeys {
		exponent := big.NewInt(int64(key.E)).Bytes()
		keys = append(keys, map[string]string{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(exponent),
		})
	}
	return map[string]interface{}{"keys": keys}
}

type jwtAccessGenerator struct{ owner *OAuth2 }

func (g jwtAccessGenerator) Token(ctx context.Context, data *gooauth2.GenerateBasic, isGenRefresh bool) (string, string, error) {
	client, err := g.owner.clients.FindByClientID(ctx, data.Client.GetID())
	if err != nil {
		return "", "", oauth2errors.ErrInvalidClient
	}
	for _, scope := range strings.Fields(data.TokenInfo.GetScope()) {
		if !slices.Contains(client.AllowedScopes, scope) {
			return "", "", oauth2errors.ErrInvalidScope
		}
	}
	user, err := db.NewUserRepository(g.owner.db).FindByID(ctx, data.UserID)
	if err != nil || !user.IsActive {
		return "", "", oauth2errors.ErrInvalidGrant
	}
	extension, ok := data.TokenInfo.(gooauth2.ExtendableTokenInfo)
	if !ok || extension.GetExtension().Get("session_id") == "" {
		return "", "", oauth2errors.ErrInvalidGrant
	}
	session, err := db.NewUserSessionRepository(g.owner.db).FindActive(ctx, extension.GetExtension().Get("session_id"), time.Now())
	if err != nil || session.UserID != data.UserID {
		return "", "", oauth2errors.ErrInvalidGrant
	}
	access, err := g.owner.signer.sign(data.UserID, client.ClientID, client.Audiences, strings.Fields(data.TokenInfo.GetScope()), data.CreateAt)
	return access, "", err
}

func (o *OAuth2) HandleJWKS(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, o.signer.jwks())
}
