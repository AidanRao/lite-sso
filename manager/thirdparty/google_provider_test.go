package thirdparty

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"sso-server/conf"
)

type googleRoundTripFunc func(*http.Request) (*http.Response, error)

func newGoogleTestIdentity(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Google test signing key"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}))
}

func googleTestClaims() googleIDClaims {
	return googleIDClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "https://accounts.google.com", Subject: "google-user-1", Audience: jwt.ClaimStrings{"google-client"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
		Email: "alice@gmail.com", EmailVerified: true, Name: "Alice", Picture: "https://example.com/avatar.png",
	}
}

func googleTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}, "Cache-Control": {"public, max-age=3600"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func Test_GoogleProvider_AuthCodeURL_UsesIdentityScopesAndCallback(t *testing.T) {
	provider := newGoogleProvider(conf.GoogleOAuthConfig{ClientID: "google-client", ClientSecret: "secret", RedirectURI: "https://sso.example.com/api/auth/third/google/callback"})
	parsed, err := url.Parse(provider.AuthCodeURL("csrf-state"))
	require.NoError(t, err)
	require.Equal(t, "https://accounts.google.com/o/oauth2/v2/auth", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	query := parsed.Query()
	require.Equal(t, "google-client", query.Get("client_id"))
	require.Equal(t, provider.cfg.RedirectURI, query.Get("redirect_uri"))
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, "csrf-state", query.Get("state"))
	require.Equal(t, "openid email profile", query.Get("scope"))
	require.NotEqual(t, "offline", query.Get("access_type"))
	require.True(t, provider.Configured())
}

func Test_GoogleProvider_FetchProfile_VerifiesIdentityAndCachesKeys(t *testing.T) {
	key, certificate := newGoogleTestIdentity(t)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, googleTestClaims())
	token.Header["kid"] = "key-1"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	certificates, err := json.Marshal(map[string]string{"key-1": certificate})
	require.NoError(t, err)
	tokenCalls, certCalls := 0, 0
	provider := newGoogleProvider(conf.GoogleOAuthConfig{ClientID: "google-client", ClientSecret: "secret", RedirectURI: "https://sso.example.com/api/auth/third/google/callback"})
	provider.client.Transport = googleRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case googleTokenURL:
			tokenCalls++
			require.Equal(t, http.MethodPost, request.Method)
			require.NoError(t, request.ParseForm())
			require.Equal(t, "authorization_code", request.Form.Get("grant_type"))
			require.Equal(t, "auth-code", request.Form.Get("code"))
			require.Equal(t, "google-client", request.Form.Get("client_id"))
			require.Equal(t, "secret", request.Form.Get("client_secret"))
			require.Equal(t, provider.cfg.RedirectURI, request.Form.Get("redirect_uri"))
			body, marshalErr := json.Marshal(map[string]string{"access_token": "access-token", "token_type": "Bearer", "id_token": signed})
			require.NoError(t, marshalErr)
			return googleTestResponse(http.StatusOK, string(body)), nil
		case googleCertsURL:
			certCalls++
			return googleTestResponse(http.StatusOK, string(certificates)), nil
		default:
			t.Fatalf("unexpected endpoint: %s", request.URL)
			return nil, nil
		}
	})
	for range 2 {
		profile, fetchErr := provider.FetchProfile(context.Background(), "auth-code")
		require.NoError(t, fetchErr)
		require.Equal(t, &Profile{Provider: Google, ProviderUID: "google-user-1", Email: "alice@gmail.com", Username: "Alice", AvatarURL: "https://example.com/avatar.png"}, profile)
	}
	require.Equal(t, 2, tokenCalls)
	require.Equal(t, 1, certCalls)
}

func Test_GoogleProvider_FetchProfile_RejectsInvalidTokens(t *testing.T) {
	key, certificate := newGoogleTestIdentity(t)
	otherKey, _ := newGoogleTestIdentity(t)
	certificates, err := json.Marshal(map[string]string{"key-1": certificate})
	require.NoError(t, err)
	for _, scenario := range []struct {
		name       string
		change     func(*googleIDClaims)
		wrongKey   bool
		missingKey bool
	}{
		{name: "wrong signature", wrongKey: true},
		{name: "missing key id", missingKey: true},
		{name: "wrong issuer", change: func(c *googleIDClaims) { c.Issuer = "https://attacker.example.com" }},
		{name: "wrong audience", change: func(c *googleIDClaims) { c.Audience = jwt.ClaimStrings{"other-client"} }},
		{name: "expired", change: func(c *googleIDClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }},
		{name: "missing expiry", change: func(c *googleIDClaims) { c.ExpiresAt = nil }},
		{name: "future issued at", change: func(c *googleIDClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }},
		{name: "missing issued at", change: func(c *googleIDClaims) { c.IssuedAt = nil }},
		{name: "missing subject", change: func(c *googleIDClaims) { c.Subject = "" }},
		{name: "wrong presenter", change: func(c *googleIDClaims) { c.AuthorizedParty = "other-client" }},
		{name: "multiple audiences without presenter", change: func(c *googleIDClaims) { c.Audience = jwt.ClaimStrings{"google-client", "other-client"} }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			claims := googleTestClaims()
			if scenario.change != nil {
				scenario.change(&claims)
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			if !scenario.missingKey {
				token.Header["kid"] = "key-1"
			}
			signingKey := key
			if scenario.wrongKey {
				signingKey = otherKey
			}
			signed, signErr := token.SignedString(signingKey)
			require.NoError(t, signErr)
			provider := newGoogleProvider(conf.GoogleOAuthConfig{ClientID: "google-client"})
			provider.client.Transport = googleRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() == googleCertsURL {
					return googleTestResponse(http.StatusOK, string(certificates)), nil
				}
				body, marshalErr := json.Marshal(map[string]string{"access_token": "access-token", "id_token": signed})
				require.NoError(t, marshalErr)
				return googleTestResponse(http.StatusOK, string(body)), nil
			})
			profile, fetchErr := provider.FetchProfile(context.Background(), "auth-code")
			require.Error(t, fetchErr)
			require.Nil(t, profile)
			require.NotContains(t, fetchErr.Error(), signed)
		})
	}
}

func Test_GoogleIDClaims_ToProfile_ImportsOnlyAuthoritativeEmail(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		email    string
		verified bool
		domain   string
		trusted  bool
	}{
		{name: "gmail", email: "alice@gmail.com", verified: true, trusted: true},
		{name: "workspace", email: "alice@example.com", verified: true, domain: "example.com", trusted: true},
		{name: "third party email", email: "alice@example.com", verified: true},
		{name: "unverified gmail", email: "alice@gmail.com"},
		{name: "unverified workspace", email: "alice@example.com", domain: "example.com"},
		{name: "gmail suffix spoof", email: "alice@notgmail.com", verified: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			claims := googleTestClaims()
			claims.Email, claims.EmailVerified, claims.HostedDomain = scenario.email, scenario.verified, scenario.domain
			profile := claims.toProfile()
			require.Equal(t, "google-user-1", profile.ProviderUID)
			if scenario.trusted {
				require.Equal(t, scenario.email, profile.Email)
			} else {
				require.Empty(t, profile.Email)
			}
		})
	}
}

func Test_GoogleProvider_FetchProfile_HandlesTokenFailureWithoutLeakingSecrets(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		status         int
		body           string
		transportError bool
	}{
		{name: "rejected code", status: http.StatusBadRequest, body: `{"error":"invalid_grant","error_description":"private-secret"}`},
		{name: "missing id token", status: http.StatusOK, body: `{"access_token":"private-secret"}`},
		{name: "malformed response", status: http.StatusOK, body: "private-secret"},
		{name: "transport failure", transportError: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			provider := newGoogleProvider(conf.GoogleOAuthConfig{ClientID: "google-client"})
			provider.client.Transport = googleRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
				if scenario.transportError {
					return nil, errors.New("private-secret")
				}
				return googleTestResponse(scenario.status, scenario.body), nil
			})
			profile, err := provider.FetchProfile(context.Background(), "private-code")
			require.Error(t, err)
			require.Nil(t, profile)
			require.NotContains(t, err.Error(), "private-secret")
			require.NotContains(t, err.Error(), "private-code")
		})
	}
}

func Test_GoogleSigningKeys_Get_RefreshesExpiredAndRotatedKeys(t *testing.T) {
	_, certificate := newGoogleTestIdentity(t)
	keys := &googleSigningKeys{}
	calls := 0
	client := &http.Client{Transport: googleRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, googleCertsURL, request.URL.String())
		id := "key-1"
		if calls > 1 {
			id = "key-2"
		}
		body, err := json.Marshal(map[string]string{id: certificate})
		require.NoError(t, err)
		return googleTestResponse(http.StatusOK, string(body)), nil
	})}
	_, err := keys.get(context.Background(), client, "key-1")
	require.NoError(t, err)
	_, err = keys.get(context.Background(), client, "key-2")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	keys.expiresAt = time.Now().Add(-time.Second)
	_, err = keys.get(context.Background(), client, "key-2")
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func Test_GoogleSigningKeys_Get_RejectsUnavailableOrMalformedKeys(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "unavailable", status: http.StatusServiceUnavailable, body: "upstream error"},
		{name: "invalid json", status: http.StatusOK, body: "invalid"},
		{name: "invalid certificate", status: http.StatusOK, body: `{"key-1":"invalid"}`},
		{name: "missing key", status: http.StatusOK, body: `{}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			client := &http.Client{Transport: googleRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return googleTestResponse(scenario.status, scenario.body), nil
			})}
			keys := &googleSigningKeys{}
			_, err := keys.get(context.Background(), client, "key-1")
			require.Error(t, err)
		})
	}
}

func Test_GoogleKeysTTL_RespectsCacheControlAndAge(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		control string
		age     string
		want    time.Duration
	}{
		{name: "cached", control: "public, max-age=3600", want: time.Hour},
		{name: "upstream age", control: "max-age=3600", age: "600", want: 50 * time.Minute},
		{name: "expired", control: "max-age=60", age: "120"},
		{name: "no store", control: "no-store, max-age=3600"},
		{name: "no cache", control: "max-age=3600, no-cache"},
		{name: "invalid", control: "max-age=invalid"},
		{name: "missing"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			require.Equal(t, scenario.want, googleKeysTTL(http.Header{"Cache-Control": {scenario.control}, "Age": {scenario.age}}))
		})
	}
}

func (f googleRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
