package db

import (
	"context"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/model"
)

// ValidateOAuthAuthorization checks the exact registration and association that issued a grant.
// Empty markers match only the still-existing legacy records migrated in place.
func ValidateOAuthAuthorization(ctx context.Context, database *gorm.DB, userID, clientID, registrationID, authorizationID string) (*model.OAuthClient, error) {
	client, err := NewOAuthClientRepository(database).FindByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client.RegistrationID != registrationID {
		return nil, common.ErrInvalidCredentials
	}
	association, err := NewUserOAuthClientRepository(database).FindAssociation(ctx, userID, clientID)
	if err != nil {
		return nil, err
	}
	if association.AuthorizationID != authorizationID {
		return nil, common.ErrInvalidCredentials
	}
	return client, nil
}
