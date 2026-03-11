package oauth2

import (
	"context"

	gooauth2 "github.com/go-oauth2/oauth2/v4"
	oauth2errors "github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/models"
	"gorm.io/gorm"

	"sso-server/dal/db"
)

type ClientStore struct {
	db *gorm.DB
}

func NewClientStore(database *gorm.DB) *ClientStore {
	return &ClientStore{db: database}
}

func (s *ClientStore) GetByID(ctx context.Context, id string) (gooauth2.ClientInfo, error) {
	if id == "" {
		return nil, oauth2errors.ErrInvalidClient
	}

	if id == "api" {
		return &models.Client{
			ID:     "api",
			Secret: "",
			Domain: `[""]`,
			Public: true,
		}, nil
	}

	clientRepo := db.NewOAuthClientRepository(s.db)
	client, err := clientRepo.FindByClientID(ctx, id)
	if err != nil {
		return nil, oauth2errors.ErrInvalidClient
	}

	return &models.Client{
		ID:     client.ClientID,
		Secret: client.ClientSecret,
		Domain: client.RedirectURIs,
	}, nil
}
