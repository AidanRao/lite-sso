package oauth2

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"sso-server/conf"
)

func Test_TokenSigner_JWKSAndExpiry(t *testing.T) {
	signer, err := newTokenSigner(conf.OAuthTokenConfig{Issuer: "https://sso.example.test", AccessTokenTTL: 30 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, SigningKID: "key-1", SigningPrivateKeyPEM: testPrivateKeyPEM(t)})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	access, err := signer.sign("usr_1", "android-app", []string{"classhopper-api", "profile-api"}, []string{"courses:read"}, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.parse(access)
	if err != nil || claims.Subject != "usr_1" || claims.ClientID != "android-app" || claims.Scope != "courses:read" || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 30*time.Minute {
		t.Fatalf("unexpected signed claims: %#v %v", claims, err)
	}
	keys := signer.jwks()["keys"].([]map[string]string)
	if len(keys) != 1 || keys[0]["kid"] != signer.kid || keys[0]["alg"] != "RS256" {
		t.Fatalf("unexpected JWKS: %#v", keys)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(keys[0]["n"])
	if err != nil {
		t.Fatal(err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(keys[0]["e"])
	if err != nil {
		t.Fatal(err)
	}
	publicKey := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(new(big.Int).SetBytes(eBytes).Int64())}
	_, err = jwt.Parse(access, func(token *jwt.Token) (interface{}, error) { return publicKey, nil }, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("https://sso.example.test"), jwt.WithAudience("classhopper-api"), jwt.WithExpirationRequired())
	if err != nil {
		t.Fatalf("JWKS public key failed to verify access token: %v", err)
	}
	expired, err := signer.sign("usr_1", "android-app", []string{"classhopper-api"}, nil, now.Add(-31*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.parse(expired); err == nil {
		t.Fatal("expired access token accepted")
	}
	if _, err := signer.parse("legacy-opaque-access-token"); err == nil {
		t.Fatal("legacy opaque token accepted")
	}
}

func Test_TokenSigner_PreviousPublicKey(t *testing.T) {
	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := func(key *rsa.PrivateKey) string {
		return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	}
	oldSigner, err := newTokenSigner(conf.OAuthTokenConfig{Issuer: "https://sso.example.test", AccessTokenTTL: 30 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, SigningKID: "key-old", SigningPrivateKeyPEM: privatePEM(oldKey)})
	if err != nil {
		t.Fatal(err)
	}
	oldToken, err := oldSigner.sign("usr_1", "android-app", []string{"classhopper-api"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	oldPublicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustPublicKeyDER(t, &oldKey.PublicKey)}))
	previous, err := json.Marshal(map[string]string{"key-old": oldPublicPEM})
	if err != nil {
		t.Fatal(err)
	}
	newSigner, err := newTokenSigner(conf.OAuthTokenConfig{Issuer: "https://sso.example.test", AccessTokenTTL: 30 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, SigningKID: "key-new", SigningPrivateKeyPEM: privatePEM(newKey), PreviousPublicKeys: string(previous)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSigner.parse(oldToken); err != nil {
		t.Fatalf("previous key failed to verify live token: %v", err)
	}
	if len(newSigner.jwks()["keys"].([]map[string]string)) != 2 {
		t.Fatal("JWKS did not publish both keys")
	}
}

func Test_TokenSigner_RequiresConfiguredPrivateKey(t *testing.T) {
	_, err := newTokenSigner(conf.OAuthTokenConfig{Issuer: "http://localhost:8080", AccessTokenTTL: 30 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, SigningKID: "key-1"})
	if err == nil {
		t.Fatal("OAuth signer accepted a missing private key")
	}
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

func mustPublicKeyDER(t *testing.T, key *rsa.PublicKey) []byte {
	t.Helper()
	encoded, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
