package auth

import (
	"context"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/model"
)

// LoginWithPassword authenticates a user with email and password
func (s *AuthService) LoginWithPassword(ctx context.Context, r *http.Request, email, password string) (*model.User, map[string]interface{}, error) {
	userRepo := db.NewUserRepository(s.db)

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, common.ErrInvalidCredentials
	}

	if user.PasswordHash == nil {
		return nil, nil, common.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, common.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, nil, common.ErrUserInactive
	}

	if s.oauth2 == nil || r == nil {
		return user, nil, nil
	}

	tokenData, err := s.oauth2.IssueTokenForUser(ctx, r, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return user, tokenData, nil
}
