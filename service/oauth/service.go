package oauth

import (
	"context"

	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
	"sso-server/handler/oauth2"
)

// UserInfo is deprecated, use dto.UserResponse instead
type UserInfo = dto.UserResponse

type OAuthService struct {
	cfg      *conf.Config
	db       *gorm.DB
	kv       kv.Store
	oauth2   *oauth2.OAuth2
	userRepo *db.UserRepository
}

func NewOAuthService(cfg *conf.Config, database *gorm.DB, kvStore kv.Store, oauth2Impl *oauth2.OAuth2, userRepo *db.UserRepository) *OAuthService {
	return &OAuthService{
		cfg:      cfg,
		db:       database,
		kv:       kvStore,
		oauth2:   oauth2Impl,
		userRepo: userRepo,
	}
}

// GetUserInfo retrieves user information - moves DAL access from handler to service layer
func (s *OAuthService) GetUserInfo(ctx context.Context, userID string) (*dto.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return dto.ToUserResponse(user), nil
}

// HandleThirdPartyLogin initiates third-party OAuth flow
func (s *OAuthService) HandleThirdPartyLogin(ctx context.Context, provider string) (string, error) {
	// TODO: Implement third-party login logic
	// This will redirect to the third-party provider
	return "", ErrInvalidProvider
}

// HandleThirdPartyCallback handles callback from third-party OAuth provider
func (s *OAuthService) HandleThirdPartyCallback(ctx context.Context, provider, code string) (*dto.UserResponse, error) {
	// TODO: Implement third-party callback logic
	// Exchange code for token, get user info, create/update user
	return nil, ErrProviderAuthFailed
}

// BindThirdPartyAccount binds a third-party account to an existing user
func (s *OAuthService) BindThirdPartyAccount(ctx context.Context, userID, provider string) error {
	// TODO: Implement third-party account binding
	return nil
}
