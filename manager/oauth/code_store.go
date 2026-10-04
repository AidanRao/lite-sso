// Package oauth manages OAuth authorization codes and refresh grants.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	gooauth2 "github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"

	"sso-server/dal/kv"
)

// ErrInvalidAuthorizationCodeToken rejects unsupported token types or expired codes.
var ErrInvalidAuthorizationCodeToken = errors.New("OAuth code store accepts authorization codes and stateless access tokens only")

// AuthorizationCodeStore adapts the OAuth library to code records needed by this service.
// Access tokens are signed JWTs, and refresh grants have a separate store.
type AuthorizationCodeStore struct {
	kv kv.Store
}

// NewAuthorizationCodeStore creates a code store on an already namespaced key-value store.
func NewAuthorizationCodeStore(store kv.Store) *AuthorizationCodeStore {
	return &AuthorizationCodeStore{kv: store}
}

func (s *AuthorizationCodeStore) Create(ctx context.Context, info gooauth2.TokenInfo) error {
	if code := info.GetCode(); code != "" {
		remaining := time.Until(info.GetCodeCreateAt().Add(info.GetCodeExpiresIn()))
		if remaining <= 0 {
			return ErrInvalidAuthorizationCodeToken
		}
		payload, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return s.kv.Set(ctx, kv.KeyOAuthAuthorizationCode(code), string(payload), remaining)
	}
	if info.GetAccess() == "" || info.GetRefresh() != "" {
		return ErrInvalidAuthorizationCodeToken
	}
	return nil
}

func (s *AuthorizationCodeStore) RemoveByCode(ctx context.Context, code string) error {
	// The OAuth manager calls this after GetByCode, which consumes the code.
	return nil
}

func (s *AuthorizationCodeStore) RemoveByAccess(ctx context.Context, access string) error {
	return ErrInvalidAuthorizationCodeToken
}

func (s *AuthorizationCodeStore) RemoveByRefresh(ctx context.Context, refresh string) error {
	return ErrInvalidAuthorizationCodeToken
}

func (s *AuthorizationCodeStore) GetByCode(ctx context.Context, code string) (gooauth2.TokenInfo, error) {
	// The manager reads each code only to exchange it. GETDEL prevents two
	// concurrent exchanges from using the same authorization code.
	payload, err := s.kv.Take(ctx, kv.KeyOAuthAuthorizationCode(code))
	if errors.Is(err, kv.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var info models.Token
	if err := json.Unmarshal([]byte(payload), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *AuthorizationCodeStore) GetByAccess(ctx context.Context, access string) (gooauth2.TokenInfo, error) {
	return nil, ErrInvalidAuthorizationCodeToken
}

func (s *AuthorizationCodeStore) GetByRefresh(ctx context.Context, refresh string) (gooauth2.TokenInfo, error) {
	return nil, ErrInvalidAuthorizationCodeToken
}
