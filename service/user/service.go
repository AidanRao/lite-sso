package user

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	serviceauth "sso-server/service/auth"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
	"sso-server/handler/oauth2"
	"sso-server/model"
)

var supportedThirdPartyProviders = []string{"github", "feishu"}

type UserService struct {
	cfg    *conf.Config
	db     *gorm.DB
	kv     kv.Store
	oauth2 *oauth2.OAuth2
}

func NewUserService(cfg *conf.Config, db *gorm.DB, kvStore kv.Store, oauth2Impl *oauth2.OAuth2) *UserService {
	return &UserService{
		cfg:    cfg,
		db:     db,
		kv:     kvStore,
		oauth2: oauth2Impl,
	}
}

func (s *UserService) RegisterWithEmailChallenge(ctx context.Context, email string, password string, username *string, challengeID string, code string, deviceID string) (*model.User, error) {
	authService := serviceauth.NewAuthService(s.cfg, s.db, s.kv, nil, s.oauth2)
	return authService.RegisterWithEmailChallenge(ctx, email, password, username, challengeID, code, deviceID)
}

func (s *UserService) CreateSession(ctx context.Context, userID string) (string, error) {
	authService := serviceauth.NewAuthService(s.cfg, s.db, s.kv, nil, s.oauth2)
	return authService.CreateSession(ctx, userID)
}

func (s *UserService) ResetPasswordWithEmailChallenge(ctx context.Context, email string, password string, challengeID string, code string, deviceID string) error {
	authService := serviceauth.NewAuthService(s.cfg, s.db, s.kv, nil, s.oauth2)
	return authService.ResetPasswordWithEmailChallenge(ctx, email, password, challengeID, code, deviceID)
}

func (s *UserService) GetProfileOverview(ctx context.Context, userID string) (*dto.ProfileResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	appRepo := db.NewUserOAuthClientRepository(s.db)
	apps, err := appRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	thirdPartyRepo := db.NewUserThirdPartyRepository(s.db)
	bindings, err := thirdPartyRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	boundProviders := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		boundProviders[binding.Provider] = true
	}

	appResponses := make([]dto.UserApplicationResponse, 0, len(apps))
	for _, app := range apps {
		appResponses = append(appResponses, dto.UserApplicationResponse{
			ClientID:    app.ClientID,
			Name:        app.Name,
			HomepageURL: app.HomepageURL,
			LastLoginAt: app.LastLoginAt,
		})
	}

	providerResponses := make([]dto.ThirdPartyProviderResponse, 0, len(supportedThirdPartyProviders))
	for _, provider := range supportedThirdPartyProviders {
		providerResponses = append(providerResponses, dto.ThirdPartyProviderResponse{
			Provider: provider,
			Bound:    boundProviders[provider],
		})
	}

	return &dto.ProfileResponse{
		User:                dto.ToUserResponse(user),
		Applications:        appResponses,
		ThirdPartyProviders: providerResponses,
		IsAdmin:             s.cfg.IsAdminUser(userID),
	}, nil
}

// UnbindThirdParty removes a third-party login method from the current user.
func (s *UserService) UnbindThirdParty(ctx context.Context, userID string, provider string) error {
	if !isSupportedThirdPartyProvider(provider) {
		return common.ErrInvalidProvider
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userRepo := db.NewUserRepository(tx)
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrUserNotFound
			}
			return err
		}
		if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
			return common.ErrEmailRequiredForUnbind
		}

		thirdPartyRepo := db.NewUserThirdPartyRepository(tx)
		binding, err := thirdPartyRepo.FindByUserID(ctx, userID, provider)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrThirdPartyNotBound
			}
			return err
		}

		return thirdPartyRepo.Delete(ctx, binding.ID)
	})
}

// UpdateProfile updates user profile
func (s *UserService) UpdateProfile(ctx context.Context, userID string, username *string, avatarURL *string) (*dto.UserResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	if username != nil {
		trimmedUsername := strings.TrimSpace(*username)
		if trimmedUsername == "" {
			user.Username = nil
		} else {
			exists, err := userRepo.ExistsUsernameExceptID(ctx, trimmedUsername, userID)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, common.ErrUsernameExists
			}
			user.Username = &trimmedUsername
		}
	}
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}

	if err := userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return dto.ToUserResponse(user), nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isSupportedThirdPartyProvider(provider string) bool {
	for _, supportedProvider := range supportedThirdPartyProviders {
		if provider == supportedProvider {
			return true
		}
	}
	return false
}
