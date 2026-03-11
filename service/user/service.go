package user

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
	"sso-server/handler/oauth2"
	"sso-server/model"
)

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

func (s *UserService) RegisterWithEmailOTP(ctx context.Context, r *http.Request, email string, password string, username *string, otp string) (*dto.UserResponse, map[string]interface{}, error) {
	if ok, err := s.verifyOTP(ctx, email, otp); err != nil || !ok {
		return nil, nil, ErrInvalidOTP
	}

	userRepo := db.NewUserRepository(s.db)

	exists, err := userRepo.ExistsEmail(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, ErrEmailExists
	}

	if username != nil && strings.TrimSpace(*username) != "" {
		exists, err := userRepo.ExistsUsername(ctx, strings.TrimSpace(*username))
		if err != nil {
			return nil, nil, err
		}
		if exists {
			return nil, nil, ErrUsernameExists
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, nil, err
	}
	hashStr := string(hash)

	user := &model.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: &hashStr,
		IsActive:     true,
	}
	if username != nil && strings.TrimSpace(*username) != "" {
		u := strings.TrimSpace(*username)
		user.Username = &u
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, nil, err
	}

	if s.oauth2 == nil || r == nil {
		return dto.ToUserResponse(user), nil, nil
	}

	tokenData, err := s.oauth2.IssueTokenForUser(ctx, r, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return dto.ToUserResponse(user), tokenData, nil
}

func (s *UserService) verifyOTP(ctx context.Context, email string, otp string) (bool, error) {
	val, err := s.kv.Get(ctx, kv.KeyOTP(email))
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(val) != strings.TrimSpace(otp) {
		return false, nil
	}
	_ = s.kv.Del(ctx, kv.KeyOTP(email))
	return true, nil
}

// GetProfile retrieves user profile
func (s *UserService) GetProfile(ctx context.Context, userID string) (*dto.UserResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return dto.ToUserResponse(user), nil
}

// UpdateProfile updates user profile
func (s *UserService) UpdateProfile(ctx context.Context, userID string, username *string, avatarURL *string) (*dto.UserResponse, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if username != nil {
		user.Username = username
	}
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}

	if err := userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return dto.ToUserResponse(user), nil
}
