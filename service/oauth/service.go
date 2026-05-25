package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

const (
	stateExpiry = 5 * time.Minute

	ThirdPartyActionLogin = "login"
	ThirdPartyActionBind  = "bind"
)

// OAuthService 编排第三方登录流程，具体平台差异由 provider 策略实现。
type OAuthService struct {
	kv                 kv.Store
	providers          map[string]thirdPartyProvider
	userRepo           *db.UserRepository
	userThirdPartyRepo *db.UserThirdPartyRepository
}

type thirdPartyState struct {
	Provider string `json:"provider"`
	Redirect string `json:"redirect"`
	Action   string `json:"action"`
	UserID   string `json:"user_id,omitempty"`
}

type ThirdPartyCallbackResult struct {
	User     *dto.UserResponse
	Redirect string
	Action   string
}

func NewOAuthService(cfg *conf.Config, database *gorm.DB, kvStore kv.Store, userRepo *db.UserRepository) *OAuthService {
	providers := map[string]thirdPartyProvider{
		githubProvider: newGitHubProvider(cfg.OAuth.GitHub),
		feishuProvider: newFeishuProvider(cfg.OAuth.Feishu),
	}

	return &OAuthService{
		kv:                 kvStore,
		providers:          providers,
		userRepo:           userRepo,
		userThirdPartyRepo: db.NewUserThirdPartyRepository(database),
	}
}

// GetUserInfo 获取用户信息，避免 handler 直接访问数据层。
func (s *OAuthService) GetUserInfo(ctx context.Context, userID string) (*dto.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	return dto.ToUserResponse(user), nil
}

// HandleThirdPartyLogin 发起第三方 OAuth 登录流程。
func (s *OAuthService) HandleThirdPartyLogin(ctx context.Context, provider string, redirect string) (string, error) {
	p, ok := s.getProvider(provider)
	if !ok {
		return "", common.ErrInvalidProvider
	}
	if !p.Configured() {
		return "", common.ErrInvalidProvider
	}

	redirectURL, err := serviceauth.NormalizeLoginRedirect(redirect)
	if err != nil {
		return "", err
	}

	state, err := generateState()
	if err != nil {
		return "", common.ErrProviderAuthFailed
	}

	stateData, err := json.Marshal(thirdPartyState{
		Provider: provider,
		Redirect: redirectURL,
		Action:   ThirdPartyActionLogin,
	})
	if err != nil {
		return "", common.ErrProviderAuthFailed
	}

	err = s.kv.Set(ctx, kv.KeyOAuthState(state), string(stateData), stateExpiry)
	if err != nil {
		return "", common.ErrProviderAuthFailed
	}

	return p.AuthCodeURL(state), nil
}

func (s *OAuthService) HandleThirdPartyBind(ctx context.Context, userID string, provider string, redirect string) (string, error) {
	p, ok := s.getProvider(provider)
	if !ok {
		return "", common.ErrInvalidProvider
	}
	if !p.Configured() {
		return "", common.ErrInvalidProvider
	}

	if userID == "" {
		return "", common.ErrUserNotFound
	}
	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		return "", common.ErrUserNotFound
	}

	redirectURL, err := serviceauth.NormalizeLoginRedirect(redirect)
	if err != nil {
		return "", err
	}

	state, err := generateState()
	if err != nil {
		return "", common.ErrProviderAuthFailed
	}

	stateData, err := json.Marshal(thirdPartyState{
		Provider: provider,
		Redirect: redirectURL,
		Action:   ThirdPartyActionBind,
		UserID:   userID,
	})
	if err != nil {
		return "", common.ErrProviderAuthFailed
	}

	if err := s.kv.Set(ctx, kv.KeyOAuthState(state), string(stateData), stateExpiry); err != nil {
		return "", common.ErrProviderAuthFailed
	}

	return p.AuthCodeURL(state), nil
}

// HandleThirdPartyCallbackWithState 校验 state 并处理第三方 OAuth 回调。
func (s *OAuthService) HandleThirdPartyCallbackWithState(ctx context.Context, provider, code, state string) (*ThirdPartyCallbackResult, error) {
	p, ok := s.getProvider(provider)
	if !ok {
		return nil, common.ErrInvalidProvider
	}

	stateData, err := s.validateState(ctx, state)
	if err != nil || stateData == nil || stateData.Provider != provider {
		log.Printf("OAuthService: invalid third party state, provider=%s, has_state=%t, state_provider=%s, err=%v", provider, state != "", stateProvider(stateData), err)
		return nil, common.ErrProviderAuthFailed
	}

	profile, err := p.FetchProfile(ctx, code)
	if err != nil {
		log.Printf("OAuthService: failed to get provider profile, provider=%s, err=%v", provider, err)
		return nil, common.ErrProviderAuthFailed
	}

	if stateData.Action == "" {
		stateData.Action = ThirdPartyActionLogin
	}

	if stateData.Action == ThirdPartyActionBind {
		user, err := s.bindThirdPartyUser(ctx, stateData.UserID, profile)
		if err != nil {
			log.Printf("OAuthService: failed to bind third party user, provider=%s, provider_uid=%s, user_id=%s, err=%v", provider, profile.ProviderUID, stateData.UserID, err)
			return nil, err
		}

		return &ThirdPartyCallbackResult{
			User:     dto.ToUserResponse(user),
			Redirect: stateData.Redirect,
			Action:   ThirdPartyActionBind,
		}, nil
	}

	user, err := s.findOrCreateUser(ctx, profile)
	if err != nil {
		log.Printf("OAuthService: failed to find or create third party user, provider=%s, provider_uid=%s, has_email=%t, err=%v", provider, profile.ProviderUID, profile.Email != "", err)
		return nil, common.ErrProviderAuthFailed
	}

	return &ThirdPartyCallbackResult{
		User:     dto.ToUserResponse(user),
		Redirect: stateData.Redirect,
		Action:   ThirdPartyActionLogin,
	}, nil
}

func (s *OAuthService) getProvider(provider string) (thirdPartyProvider, bool) {
	p, ok := s.providers[provider]
	if !ok || p == nil {
		return nil, false
	}
	return p, true
}

func (s *OAuthService) validateState(ctx context.Context, state string) (*thirdPartyState, error) {
	if state == "" {
		return nil, nil
	}

	key := kv.KeyOAuthState(state)
	raw, err := s.kv.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	_ = s.kv.Del(ctx, key)

	var stateData thirdPartyState
	if err := json.Unmarshal([]byte(raw), &stateData); err != nil {
		return nil, err
	}

	return &stateData, nil
}

func (s *OAuthService) findOrCreateUser(ctx context.Context, profile *thirdPartyProfile) (*model.User, error) {
	if profile == nil || profile.Provider == "" || profile.ProviderUID == "" {
		return nil, common.ErrProviderAuthFailed
	}

	binding, err := s.userThirdPartyRepo.FindByProviderUID(ctx, profile.Provider, profile.ProviderUID)
	if err == nil && binding != nil {
		user, err := s.userRepo.FindByID(ctx, binding.UserID)
		if err != nil {
			return nil, err
		}
		return user, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if profile.Email != "" {
		user, err := s.userRepo.FindByEmail(ctx, profile.Email)
		if err == nil && user != nil {
			err = s.createThirdPartyBinding(ctx, user.ID, profile.Provider, profile.ProviderUID)
			if err != nil {
				return nil, err
			}
			return user, nil
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	userID := generateUserID()
	user := &model.User{
		ID:        userID,
		Username:  stringPtr(profile.Username),
		Email:     stringPtr(profile.Email),
		AvatarURL: stringPtr(profile.AvatarURL),
		IsActive:  true,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	err = s.createThirdPartyBinding(ctx, userID, profile.Provider, profile.ProviderUID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *OAuthService) createThirdPartyBinding(ctx context.Context, userID, provider, providerUID string) error {
	binding := &model.UserThirdParty{
		UserID:      userID,
		Provider:    provider,
		ProviderUID: providerUID,
	}
	return s.userThirdPartyRepo.Create(ctx, binding)
}

func (s *OAuthService) bindThirdPartyUser(ctx context.Context, userID string, profile *thirdPartyProfile) (*model.User, error) {
	if userID == "" {
		return nil, common.ErrUserNotFound
	}
	if profile == nil || profile.Provider == "" || profile.ProviderUID == "" {
		return nil, common.ErrProviderAuthFailed
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, common.ErrUserNotFound
	}

	binding, err := s.userThirdPartyRepo.FindByProviderUID(ctx, profile.Provider, profile.ProviderUID)
	if err == nil && binding != nil {
		if binding.UserID == userID {
			return user, common.ErrThirdPartyAlreadyBound
		}
		return nil, common.ErrThirdPartyBoundToAnother
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	currentBinding, err := s.userThirdPartyRepo.FindByUserID(ctx, userID, profile.Provider)
	if err == nil && currentBinding != nil {
		return user, common.ErrThirdPartyAlreadyBound
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := s.createThirdPartyBinding(ctx, userID, profile.Provider, profile.ProviderUID); err != nil {
		return nil, err
	}

	return user, nil
}

func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func generateUserID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("u%x", b)
}

func stateProvider(stateData *thirdPartyState) string {
	if stateData == nil {
		return ""
	}
	return stateData.Provider
}
