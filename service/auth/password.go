package auth

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/model"
)

// LoginWithPassword authenticates a user with email and password
func (s *AuthService) LoginWithPassword(ctx context.Context, email, password string) (*model.User, error) {
	userRepo := db.NewUserRepository(s.db)

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, common.ErrInvalidCredentials
	}

	if user.PasswordHash == nil {
		return nil, common.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, common.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, common.ErrUserInactive
	}

	return user, nil
}
