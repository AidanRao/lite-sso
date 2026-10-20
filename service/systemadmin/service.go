// Package systemadmin contains system administration business logic.
package systemadmin

import (
	"context"
	"errors"
	"io"
	"log"
	"net/url"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dto"
	manageross "sso-server/manager/oss"
	"sso-server/manager/thirdparty"
	"sso-server/model"
)

type AdminService struct {
	database           *gorm.DB
	cfg                *conf.Config
	userRepo           *db.UserRepository
	clientRepo         *db.OAuthClientRepository
	userOAuthRepo      *db.UserOAuthClientRepository
	userThirdPartyRepo *db.UserThirdPartyRepository
	imageStore         manageross.ImageStore
	registry           *thirdparty.Registry
}

// NewAdminService creates a service for system administration workflows.
func NewAdminService(cfg *conf.Config, database *gorm.DB, imageStore manageross.ImageStore, registries ...*thirdparty.Registry) *AdminService {
	var registry *thirdparty.Registry
	if len(registries) > 0 && registries[0] != nil {
		registry = registries[0]
	} else {
		registry = thirdparty.NewBuiltins(cfg)
	}
	return &AdminService{
		database:           database,
		cfg:                cfg,
		userRepo:           db.NewUserRepository(database),
		clientRepo:         db.NewOAuthClientRepository(database),
		userOAuthRepo:      db.NewUserOAuthClientRepository(database),
		userThirdPartyRepo: db.NewUserThirdPartyRepository(database),
		imageStore:         imageStore,
		registry:           registry,
	}
}

// ListUsers returns all system users with administrator markers.
func (s *AdminService) ListUsers(ctx context.Context) ([]dto.AdminUserResponse, error) {
	users, err := s.userRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.AdminUserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, *toAdminUserResponse(&user, s.cfg))
	}
	return responses, nil
}

// GetUserDetail returns a user's profile overview for administrators.
func (s *AdminService) GetUserDetail(ctx context.Context, userID string) (*dto.AdminUserDetailResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	apps, err := s.userOAuthRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	bindings, err := s.userThirdPartyRepo.ListByUserID(ctx, userID)
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
			LogoURL:     app.LogoURL,
			LastLoginAt: app.LastLoginAt,
		})
	}

	providers := s.registry.Names()
	providerResponses := make([]dto.ThirdPartyProviderResponse, 0, len(providers))
	for _, provider := range providers {
		providerResponses = append(providerResponses, dto.ThirdPartyProviderResponse{
			Provider: provider,
			Bound:    boundProviders[provider],
		})
	}

	return &dto.AdminUserDetailResponse{
		User:                toAdminUserResponse(user, s.cfg),
		Applications:        appResponses,
		ThirdPartyProviders: providerResponses,
	}, nil
}

// UploadOAuthClientLogo uploads and assigns a logo for an OAuth client.
func (s *AdminService) UploadOAuthClientLogo(ctx context.Context, id uint, contentType string, extension string, body io.Reader, size int64) (*dto.OAuthClientResponse, error) {
	if s.imageStore == nil {
		return nil, common.ErrLogoStorageUnavailable
	}

	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.ErrOAuthClientNotFound
	}

	startedAt := time.Now()
	log.Printf("oauth client logo upload started: client_id=%d content_type=%s size_bytes=%d", id, contentType, size)
	objectKey, logoURL, err := s.imageStore.UploadImage(ctx, contentType, extension, body, size)
	if err != nil {
		log.Printf("oauth client logo upload failed: client_id=%d stage=oss_put duration_ms=%d err=%v", id, time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	log.Printf("oauth client logo upload oss completed: client_id=%d duration_ms=%d", id, time.Since(startedAt).Milliseconds())

	var previousObjectKey *string
	err = s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if err != nil {
			return clientLookupError(err)
		}
		previousObjectKey = current.LogoObjectKey
		current.LogoURL, current.LogoObjectKey = &logoURL, &objectKey
		client = current
		return db.NewOAuthClientRepository(tx).UpdateLogo(ctx, current)
	})
	if err != nil {
		cleanupFailed := false
		if deleteErr := s.imageStore.DeleteImage(ctx, objectKey); deleteErr != nil {
			cleanupFailed = true
		}
		log.Printf("oauth client logo upload failed: client_id=%d stage=database_update duration_ms=%d cleanup_new_object_failed=%t err=%v", id, time.Since(startedAt).Milliseconds(), cleanupFailed, err)
		return nil, err
	}

	previousObjectCleanupFailed := false
	if previousObjectKey != nil && *previousObjectKey != "" && *previousObjectKey != objectKey {
		if err := s.imageStore.DeleteImage(ctx, *previousObjectKey); err != nil {
			previousObjectCleanupFailed = true
		}
	}
	log.Printf("oauth client logo upload completed: client_id=%d duration_ms=%d previous_object_cleanup_failed=%t", id, time.Since(startedAt).Milliseconds(), previousObjectCleanupFailed)

	response := toOAuthClientResponse(client)
	return &response, nil
}

// ClearOAuthClientLogo removes an OAuth client's logo assignment.
func (s *AdminService) ClearOAuthClientLogo(ctx context.Context, id uint) (*dto.OAuthClientResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.ErrOAuthClientNotFound
	}

	var previousObjectKey *string
	err = s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if err != nil {
			return clientLookupError(err)
		}
		previousObjectKey = current.LogoObjectKey
		current.LogoURL, current.LogoObjectKey = nil, nil
		client = current
		return db.NewOAuthClientRepository(tx).UpdateLogo(ctx, current)
	})
	if err != nil {
		return nil, err
	}

	if s.imageStore != nil && previousObjectKey != nil && *previousObjectKey != "" {
		if err := s.imageStore.DeleteImage(ctx, *previousObjectKey); err != nil {
			log.Printf("oauth client logo clear cleanup failed: stage=delete_previous")
		}
	}

	response := toOAuthClientResponse(client)
	return &response, nil
}

// ListOAuthClients returns configured OAuth clients for connected platforms.
func (s *AdminService) ListOAuthClients(ctx context.Context) ([]dto.OAuthClientResponse, error) {
	clients, err := s.clientRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.OAuthClientResponse, 0, len(clients))
	for _, client := range clients {
		responses = append(responses, toOAuthClientResponse(&client))
	}
	return responses, nil
}

// GetOAuthClientSecret returns an OAuth client secret for administrators.
func (s *AdminService) GetOAuthClientSecret(ctx context.Context, id uint) (*dto.OAuthClientSecretResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, id)
	if err != nil {
		return nil, clientLookupError(err)
	}
	if client.ClientType.IsPublic() {
		return nil, common.ErrInvalidOAuthClient
	}

	return &dto.OAuthClientSecretResponse{
		ID:           client.ID,
		ClientID:     client.ClientID,
		ClientSecret: client.ClientSecret,
	}, nil
}

func toAdminUserResponse(user *model.User, cfg *conf.Config) *dto.AdminUserResponse {
	return &dto.AdminUserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
		Status:    user.Status,
		IsAdmin:   cfg.IsAdminUser(user.ID),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// CreateOAuthClient validates and creates an OAuth client.
func (s *AdminService) CreateOAuthClient(ctx context.Context, req dto.CreateOAuthClientRequest) (*dto.OAuthClientResponse, error) {
	name := strings.TrimSpace(req.Name)
	clientID := strings.TrimSpace(req.ClientID)
	clientSecret := strings.TrimSpace(req.ClientSecret)
	clientType := model.OAuthClientType(strings.TrimSpace(req.ClientType))
	if clientType == "" {
		clientType = model.OAuthClientTypeConfidential
	}
	homepageURL, err := normalizeURI(req.HomepageURL)
	if err != nil {
		return nil, err
	}
	redirectURIs, err := normalizeRedirectURIs(req.RedirectURIs)
	if err != nil {
		return nil, err
	}
	logoutURI, err := normalizeOptionalURI(req.LogoutURI)
	if err != nil {
		return nil, err
	}
	if !clientType.IsValid() || name == "" || clientID == "" || (!clientType.IsPublic() && clientSecret == "") {
		return nil, common.ErrInvalidOAuthClient
	}
	if clientType.IsPublic() {
		clientSecret = ""
	}
	audiences, err := normalizeClaimValues(req.Audiences)
	if err != nil {
		return nil, err
	}
	scopes, err := normalizeClaimValues(req.AllowedScopes)
	if err != nil {
		return nil, err
	}

	exists, err := s.clientRepo.ExistsClientID(ctx, clientID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrOAuthClientExists
	}

	client := &model.OAuthClient{
		Name:          name,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		ClientType:    clientType,
		Audiences:     audiences,
		AllowedScopes: scopes,
		HomepageURL:   homepageURL,
		RedirectURIs:  redirectURIs,
		LogoutURI:     logoutURI,
	}
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}
	response := toOAuthClientResponse(client)
	return &response, nil
}

// UpdateOAuthClient validates and updates an OAuth client.
func (s *AdminService) UpdateOAuthClient(ctx context.Context, id uint, req dto.UpdateOAuthClientRequest) (*dto.OAuthClientResponse, error) {
	var response dto.OAuthClientResponse
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := db.NewOAuthClientRepository(tx).LockByID(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrOAuthClientNotFound
		}
		if err != nil {
			return err
		}

		name := strings.TrimSpace(req.Name)
		clientType := client.ClientType
		if req.ClientType != nil {
			clientType = model.OAuthClientType(strings.TrimSpace(*req.ClientType))
		}
		homepageURL, err := normalizeURI(req.HomepageURL)
		if err != nil {
			return err
		}
		redirectURIs, err := normalizeRedirectURIs(req.RedirectURIs)
		if err != nil {
			return err
		}
		logoutURI, err := normalizeOptionalURI(req.LogoutURI)
		if err != nil {
			return err
		}
		if name == "" || !clientType.IsValid() {
			return common.ErrInvalidOAuthClient
		}
		audiences, err := normalizeClaimValues(req.Audiences)
		if err != nil {
			return err
		}
		scopes, err := normalizeClaimValues(req.AllowedScopes)
		if err != nil {
			return err
		}

		client.Name = name
		client.HomepageURL = homepageURL
		client.RedirectURIs = redirectURIs
		client.LogoutURI = logoutURI
		client.ClientType = clientType
		client.Audiences = audiences
		client.AllowedScopes = scopes
		if clientType.IsPublic() {
			client.ClientSecret = ""
		} else if client.ClientSecret == "" {
			client.ClientSecret, err = newClientSecret()
			if err != nil {
				return err
			}
		}
		if !clientType.IsPublic() && client.ClientSecret == "" {
			return common.ErrInvalidOAuthClient
		}

		if err := tx.Model(&model.OAuthClient{}).Where("id = ?", client.ID).Update("client_secret", client.ClientSecret).Error; err != nil {
			return err
		}
		if err := db.NewOAuthClientRepository(tx).Update(ctx, client); err != nil {
			return err
		}
		response = toOAuthClientResponse(client)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func toOAuthClientResponse(client *model.OAuthClient) dto.OAuthClientResponse {
	return dto.OAuthClientResponse{
		ID:            client.ID,
		Name:          client.Name,
		ClientID:      client.ClientID,
		ClientType:    string(client.ClientType),
		Audiences:     append([]string{}, client.Audiences...),
		AllowedScopes: append([]string{}, client.AllowedScopes...),
		HomepageURL:   client.HomepageURL,
		RedirectURIs:  append([]string{}, client.RedirectURIs...),
		LogoutURI:     client.LogoutURI,
		LogoURL:       client.LogoURL,
	}
}

func normalizeClaimValues(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, " \t\r\n") || len(value) > 128 {
			return nil, common.ErrInvalidOAuthClient
		}
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result, nil
}

func normalizeRedirectURIs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, common.ErrInvalidOAuthClient
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized, err := normalizeURI(value)
		if err != nil || strings.ContainsAny(normalized, "#\r\n\t ") {
			return nil, common.ErrInvalidOAuthClient
		}
		parsed, err := url.Parse(normalized)
		if err != nil || parsed.User != nil {
			return nil, common.ErrInvalidOAuthClient
		}
		if !slices.Contains(result, normalized) {
			result = append(result, normalized)
		}
	}
	return result, nil
}

func normalizeURI(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", common.ErrInvalidOAuthClient
	}
	return trimmed, nil
}

func normalizeOptionalURI(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	return normalizeURI(trimmed)
}
