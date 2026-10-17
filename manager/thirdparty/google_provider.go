package thirdparty

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"sso-server/conf"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleCertsURL = "https://www.googleapis.com/oauth2/v1/certs"
)

type googleOAuthProvider struct {
	cfg    conf.GoogleOAuthConfig
	client *http.Client
	keys   googleSigningKeys
}

type googleIDClaims struct {
	jwt.RegisteredClaims
	AuthorizedParty string `json:"azp"`
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	HostedDomain    string `json:"hd"`
	Name            string `json:"name"`
	Picture         string `json:"picture"`
}

type googleSigningKeys struct {
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

func newGoogleProvider(cfg conf.GoogleOAuthConfig) *googleOAuthProvider {
	return &googleOAuthProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func googleKeysTTL(header http.Header) time.Duration {
	var ttl time.Duration
	for _, directive := range strings.Split(header.Get("Cache-Control"), ",") {
		name, value, _ := strings.Cut(strings.TrimSpace(directive), "=")
		switch strings.ToLower(name) {
		case "no-store", "no-cache":
			return 0
		case "max-age":
			seconds, err := strconv.ParseUint(strings.Trim(value, "\""), 10, 32)
			if err == nil {
				ttl = min(time.Duration(seconds)*time.Second, 24*time.Hour)
			}
		}
	}
	age, err := strconv.ParseUint(header.Get("Age"), 10, 32)
	if err == nil {
		ttl -= time.Duration(age) * time.Second
	}
	return max(ttl, 0)
}

func (p *googleOAuthProvider) Configured() bool {
	return p.cfg.IsConfigured()
}

func (p *googleOAuthProvider) AuthCodeURL(state string) string {
	return p.oauthConfig().AuthCodeURL(state)
}

func (p *googleOAuthProvider) ManagementURL() string {
	return "https://myaccount.google.com/connections"
}

func (p *googleOAuthProvider) FetchProfile(ctx context.Context, code string) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	token, err := p.oauthConfig().Exchange(context.WithValue(ctx, oauth2.HTTPClient, p.client), code)
	if err != nil {
		// OAuth errors can contain credentials or token response bodies.
		return nil, fmt.Errorf("google token exchange failed")
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("google token response missing id_token")
	}
	claims := &googleIDClaims{}
	_, err = jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (any, error) {
		keyID, _ := token.Header["kid"].(string)
		return p.keys.get(ctx, p.client, keyID)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(p.cfg.ClientID), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return nil, fmt.Errorf("google id_token verification failed")
	}
	if (claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com") || strings.TrimSpace(claims.Subject) == "" || claims.IssuedAt == nil {
		return nil, fmt.Errorf("google id_token contains invalid identity claims")
	}
	if (claims.AuthorizedParty != "" || len(claims.Audience) > 1) && claims.AuthorizedParty != p.cfg.ClientID {
		return nil, fmt.Errorf("google id_token presenter mismatch")
	}
	return claims.toProfile(), nil
}

func (p *googleOAuthProvider) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID: p.cfg.ClientID, ClientSecret: p.cfg.ClientSecret, RedirectURL: p.cfg.RedirectURI,
		Scopes:   []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{AuthURL: googleAuthURL, TokenURL: googleTokenURL, AuthStyle: oauth2.AuthStyleInParams},
	}
}

func (c *googleIDClaims) toProfile() *Profile {
	email := ""
	// Google only remains authoritative for Gmail and verified Workspace email.
	if c.EmailVerified && (strings.HasSuffix(strings.ToLower(c.Email), "@gmail.com") || strings.TrimSpace(c.HostedDomain) != "") {
		email = c.Email
	}
	return &Profile{Provider: Google, ProviderUID: c.Subject, Email: email, Username: c.Name, AvatarURL: c.Picture}
}

func (k *googleSigningKeys) get(ctx context.Context, client *http.Client, keyID string) (*rsa.PublicKey, error) {
	if keyID == "" {
		return nil, fmt.Errorf("google id_token missing signing key id")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if key := k.keys[keyID]; key != nil && time.Now().Before(k.expiresAt) {
		return key, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, googleCertsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google signing key request failed")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("google signing key lookup failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google signing key lookup returned http status %d", response.StatusCode)
	}
	const maxResponseSize = 64 * 1024
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil || len(body) > maxResponseSize {
		return nil, fmt.Errorf("google signing key response unreadable or too large")
	}
	var certificates map[string]string
	if err := json.Unmarshal(body, &certificates); err != nil {
		return nil, fmt.Errorf("google signing key response invalid")
	}
	keys := make(map[string]*rsa.PublicKey, len(certificates))
	for id, certificate := range certificates {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certificate))
		if err != nil {
			return nil, fmt.Errorf("google signing certificate invalid")
		}
		keys[id] = key
	}
	k.keys = keys
	k.expiresAt = time.Now().Add(googleKeysTTL(response.Header))
	key := k.keys[keyID]
	if key == nil {
		return nil, fmt.Errorf("google signing key not found")
	}
	return key, nil
}
