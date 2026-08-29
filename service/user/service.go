package user

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	serviceauth "sso-server/service/auth"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
	"sso-server/handler/oauth2"
	manageross "sso-server/manager/oss"
	"sso-server/model"
)

var supportedThirdPartyProviders = []string{"github", "feishu"}

type UserService struct {
	cfg        *conf.Config
	db         *gorm.DB
	kv         kv.Store
	oauth2     *oauth2.OAuth2
	imageStore manageross.ImageStore
}

func NewUserService(cfg *conf.Config, db *gorm.DB, kvStore kv.Store, oauth2Impl *oauth2.OAuth2, imageStore manageross.ImageStore) *UserService {
	return &UserService{
		cfg:        cfg,
		db:         db,
		kv:         kvStore,
		oauth2:     oauth2Impl,
		imageStore: imageStore,
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

// GetProfile returns the current user's account summary.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*dto.ProfileResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	return &dto.ProfileResponse{
		User:    dto.ToUserResponse(user),
		IsAdmin: s.cfg.IsAdminUser(userID),
	}, nil
}

// GetLoginMethods returns only sign-in methods that are usable by the current user.
func (s *UserService) GetLoginMethods(ctx context.Context, userID string) ([]dto.LoginMethodResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	methods := make([]dto.LoginMethodResponse, 0, 4)
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		methods = append(methods, dto.LoginMethodResponse{
			Type:  dto.LoginMethodEmailOTP,
			Email: user.Email,
		})
	}
	if user.PasswordHash != nil && strings.TrimSpace(*user.PasswordHash) != "" {
		methods = append(methods, dto.LoginMethodResponse{Type: dto.LoginMethodPassword})
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

	for _, provider := range supportedThirdPartyProviders {
		if !s.isThirdPartyProviderConfigured(provider) {
			continue
		}
		bound := boundProviders[provider]
		methods = append(methods, dto.LoginMethodResponse{
			Type:     dto.LoginMethodThirdParty,
			Provider: provider,
			Bound:    &bound,
		})
	}
	return methods, nil
}

// GetApplications returns OAuth clients the current user has signed in to.
func (s *UserService) GetApplications(ctx context.Context, userID string) ([]dto.UserApplicationResponse, error) {
	if _, err := db.NewUserRepository(s.db).FindByID(ctx, userID); err != nil {
		return nil, common.ErrUserNotFound
	}

	apps, err := db.NewUserOAuthClientRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.UserApplicationResponse, 0, len(apps))
	for _, app := range apps {
		responses = append(responses, dto.UserApplicationResponse{
			ClientID:    app.ClientID,
			Name:        app.Name,
			HomepageURL: app.HomepageURL,
			LogoURL:     app.LogoURL,
			LastLoginAt: app.LastLoginAt,
		})
	}
	return responses, nil
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

// UpdateProfile updates user profile.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, username *string) (*dto.UserResponse, error) {
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
	if err := userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return dto.ToUserResponse(user), nil
}

// UploadAvatar uploads and assigns a user avatar.
func (s *UserService) UploadAvatar(ctx context.Context, userID string, contentType string, extension string, body io.Reader, size int64) (*dto.UserResponse, error) {
	if s.imageStore == nil {
		return nil, common.ErrAvatarStorageUnavailable
	}

	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	startedAt := time.Now()
	log.Printf("avatar upload started: content_type=%s size_bytes=%d", contentType, size)
	objectKey, avatarURL, err := s.imageStore.UploadImage(ctx, contentType, extension, body, size)
	if err != nil {
		log.Printf("avatar upload failed: stage=oss_put duration_ms=%d err=%v", time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}

	previousObjectKey := user.AvatarObjectKey
	user.AvatarURL = &avatarURL
	user.AvatarObjectKey = &objectKey
	if err := userRepo.Update(ctx, user); err != nil {
		cleanupFailed := false
		if deleteErr := s.imageStore.DeleteImage(ctx, objectKey); deleteErr != nil {
			cleanupFailed = true
		}
		log.Printf("avatar upload failed: stage=database_update duration_ms=%d cleanup_new_object_failed=%t err=%v", time.Since(startedAt).Milliseconds(), cleanupFailed, err)
		return nil, err
	}

	previousObjectCleanupFailed := false
	if previousObjectKey != nil && *previousObjectKey != "" && *previousObjectKey != objectKey {
		if err := s.imageStore.DeleteImage(ctx, *previousObjectKey); err != nil {
			previousObjectCleanupFailed = true
		}
	}
	log.Printf("avatar upload completed: duration_ms=%d previous_object_cleanup_failed=%t", time.Since(startedAt).Milliseconds(), previousObjectCleanupFailed)

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

func (s *UserService) isThirdPartyProviderConfigured(provider string) bool {
	if s.cfg == nil {
		return false
	}
	switch provider {
	case "github":
		return s.cfg.OAuth.GitHub.IsConfigured()
	case "feishu":
		return s.cfg.OAuth.Feishu.IsConfigured()
	default:
		return false
	}
}
