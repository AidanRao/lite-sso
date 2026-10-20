package systemadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
)

// ClientHasUsersError prevents deleting a client with remaining associations.
type ClientHasUsersError struct{ UserCount int64 }

func newClientSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func clientLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrOAuthClientNotFound
	}
	return err
}

func (e *ClientHasUsersError) Error() string { return "client still has associated users" }

// GetOAuthClient returns configuration and association count without secret metadata.
func (s *AdminService) GetOAuthClient(ctx context.Context, id uint) (*dto.OAuthClientDetailResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, clientLookupError(err)
	}
	count, err := s.userOAuthRepo.CountByClientID(ctx, client.ClientID)
	if err != nil {
		return nil, err
	}
	return &dto.OAuthClientDetailResponse{OAuthClientResponse: toOAuthClientResponse(client), UserCount: count}, nil
}

// ListOAuthClientUsers returns a bounded, deterministically ordered association page.
func (s *AdminService) ListOAuthClientUsers(ctx context.Context, id uint, page, pageSize int) (*dto.OAuthClientUsersPage, error) {
	if page < 1 || page > 1000000 || pageSize < 1 || pageSize > 100 {
		return nil, common.ErrInvalidOAuthClient
	}
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, clientLookupError(err)
	}
	total, err := s.userOAuthRepo.CountByClientID(ctx, client.ClientID)
	if err != nil {
		return nil, err
	}
	rows, err := s.userOAuthRepo.PageByClientID(ctx, client.ClientID, page, pageSize)
	if err != nil {
		return nil, err
	}
	users := make([]dto.OAuthClientUserResponse, 0, len(rows))
	for _, row := range rows {
		users = append(users, dto.OAuthClientUserResponse{UserID: row.UserID, Username: row.Username, Email: row.Email, AvatarURL: row.AvatarURL, LastLoginAt: row.LastLoginAt})
	}
	return &dto.OAuthClientUsersPage{Users: users, Total: total, Page: page, PageSize: pageSize}, nil
}

// RemoveOAuthClientUsers revokes one association or every association of this client.
func (s *AdminService) RemoveOAuthClientUsers(ctx context.Context, id uint, userID string) (*dto.OAuthClientUsersRemovalResponse, error) {
	var count int64
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if err != nil {
			return clientLookupError(err)
		}
		query := tx.Where("client_id = ?", client.ClientID)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		result := query.Delete(&model.UserOAuthClient{})
		count = result.RowsAffected
		return result.Error
	})
	if err != nil {
		return nil, err
	}
	return &dto.OAuthClientUsersRemovalResponse{RemovedCount: count}, nil
}

// RotateOAuthClientSecret replaces the one active secret under the client lock.
func (s *AdminService) RotateOAuthClientSecret(ctx context.Context, id uint) (*dto.OAuthClientSecretResponse, error) {
	var response dto.OAuthClientSecretResponse
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if err != nil {
			return clientLookupError(err)
		}
		if client.ClientType.IsPublic() {
			return common.ErrInvalidOAuthClient
		}
		secret, err := newClientSecret()
		if err != nil {
			return err
		}
		if err := tx.Model(&model.OAuthClient{}).Where("id = ?", id).Update("client_secret", secret).Error; err != nil {
			return err
		}
		response = dto.OAuthClientSecretResponse{ID: id, ClientID: client.ClientID, ClientSecret: secret}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteOAuthClient permanently removes only an empty client; its ID may be registered again.
func (s *AdminService) DeleteOAuthClient(ctx context.Context, id uint) (*dto.OAuthClientResponse, error) {
	var client *model.OAuthClient
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		client, err = db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if err != nil {
			return clientLookupError(err)
		}
		count, err := db.NewUserOAuthClientRepository(tx).CountByClientID(ctx, client.ClientID)
		if err != nil {
			return err
		}
		if count > 0 {
			return &ClientHasUsersError{UserCount: count}
		}
		return tx.Delete(client).Error
	})
	if err != nil {
		return nil, err
	}
	if s.imageStore != nil && client.LogoObjectKey != nil && *client.LogoObjectKey != "" {
		if err := s.imageStore.DeleteImage(ctx, *client.LogoObjectKey); err != nil {
			log.Printf("oauth client deleted but logo cleanup failed: client_id=%d", id)
		}
	}
	response := toOAuthClientResponse(client)
	return &response, nil
}
