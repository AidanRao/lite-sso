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
	stateExpiry          = 5 * time.Minute
	pendingBindingExpiry = 5 * time.Minute

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
	DeviceID string `json:"device_id,omitempty"`
}

type pendingThirdPartyBinding struct {
	UserID   string            `json:"user_id"`
	Profile  thirdPartyProfile `json:"profile"`
	Redirect string            `json:"redirect"`
}

type ThirdPartyCallbackResult struct {
	User             *dto.UserResponse
	Redirect         string
	Action           string
	DeviceID         string
	PendingBindingID string
}

// ThirdPartyBindingPreview contains the provider profile awaiting user confirmation.
type ThirdPartyBindingPreview struct {
	Provider  string `json:"provider"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
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
	return s.HandleThirdPartyLoginWithDevice(ctx, provider, redirect, "")
}

func (s *OAuthService) HandleThirdPartyLoginWithDevice(ctx context.Context, provider string, redirect string, deviceID string) (string, error) {
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
		DeviceID: deviceID,
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
	return s.HandleThirdPartyBindWithDevice(ctx, userID, provider, redirect, "")
}

func (s *OAuthService) HandleThirdPartyBindWithDevice(ctx context.Context, userID string, provider string, redirect string, deviceID string) (string, error) {
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
		DeviceID: deviceID,
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
		user, err := s.validateThirdPartyBinding(ctx, stateData.UserID, profile)
		if err != nil {
			log.Printf("OAuthService: failed to prepare third party binding, provider=%s, provider_uid=%s, user_id=%s, err=%v", provider, profile.ProviderUID, stateData.UserID, err)
			return nil, err
		}
		bindingID, err := s.createPendingThirdPartyBinding(ctx, stateData.UserID, profile, stateData.Redirect)
		if err != nil {
			log.Printf("OAuthService: failed to save pending third party binding, provider=%s, user_id=%s, err=%v", provider, stateData.UserID, err)
			return nil, common.ErrProviderAuthFailed
		}

		return &ThirdPartyCallbackResult{
			User:             dto.ToUserResponse(user),
			Redirect:         stateData.Redirect,
			Action:           ThirdPartyActionBind,
			DeviceID:         stateData.DeviceID,
			PendingBindingID: bindingID,
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
		DeviceID: stateData.DeviceID,
	}, nil
}

// GetThirdPartyBindingPreview returns a pending binding owned by userID.
func (s *OAuthService) GetThirdPartyBindingPreview(ctx context.Context, userID, bindingID string) (*ThirdPartyBindingPreview, error) {
	pending, err := s.loadPendingThirdPartyBinding(ctx, userID, bindingID)
	if err != nil {
		return nil, err
	}

	return &ThirdPartyBindingPreview{
		Provider:  pending.Profile.Provider,
		Username:  pending.Profile.Username,
		Email:     pending.Profile.Email,
		AvatarURL: pending.Profile.AvatarURL,
	}, nil
}

// ConfirmThirdPartyBinding persists a previously previewed third-party binding.
func (s *OAuthService) ConfirmThirdPartyBinding(ctx context.Context, userID, bindingID string) (string, error) {
	pending, err := s.loadPendingThirdPartyBinding(ctx, userID, bindingID)
	if err != nil {
		return "", err
	}

	if _, err := s.bindThirdPartyUser(ctx, userID, &pending.Profile); err != nil {
		return "", err
	}
	if err := s.kv.Del(ctx, kv.KeyOAuthPendingBinding(bindingID)); err != nil {
		log.Printf("OAuthService: failed to delete confirmed third party binding, binding_id=%s, err=%v", bindingID, err)
	}

	return pending.Redirect, nil
}

func (s *OAuthService) getProvider(provider string) (thirdPartyProvider, bool) {
	p, ok := s.providers[provider]
	if !ok || p == nil {
		return nil, false
	}
	return p, true
}

func (s *OAuthService) validateState(ctx context.Context, state string) (*thirdPartyState, error) {
	stateData, err := s.loadThirdPartyState(ctx, state)
	if err != nil || stateData == nil {
		return stateData, err
	}

	_ = s.kv.Del(ctx, kv.KeyOAuthState(state))
	return stateData, nil
}

// IsThirdPartyBindingState reports whether state belongs to a binding flow without consuming it.
func (s *OAuthService) IsThirdPartyBindingState(ctx context.Context, provider, state string) bool {
	stateData, err := s.loadThirdPartyState(ctx, state)
	return err == nil && stateData != nil && stateData.Provider == provider && stateData.Action == ThirdPartyActionBind
}

func (s *OAuthService) loadThirdPartyState(ctx context.Context, state string) (*thirdPartyState, error) {
	if state == "" {
		return nil, nil
	}

	raw, err := s.kv.Get(ctx, kv.KeyOAuthState(state))
	if err != nil {
		return nil, err
	}

	var stateData thirdPartyState
	if err := json.Unmarshal([]byte(raw), &stateData); err != nil {
		return nil, err
	}

	return &stateData, nil
}

func (s *OAuthService) createPendingThirdPartyBinding(ctx context.Context, userID string, profile *thirdPartyProfile, redirect string) (string, error) {
	if profile == nil {
		return "", common.ErrProviderAuthFailed
	}
	bindingID, err := generateState()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(pendingThirdPartyBinding{
		UserID:   userID,
		Profile:  *profile,
		Redirect: redirect,
	})
	if err != nil {
		return "", err
	}
	if err := s.kv.Set(ctx, kv.KeyOAuthPendingBinding(bindingID), string(raw), pendingBindingExpiry); err != nil {
		return "", err
	}
	return bindingID, nil
}

func (s *OAuthService) loadPendingThirdPartyBinding(ctx context.Context, userID, bindingID string) (*pendingThirdPartyBinding, error) {
	if userID == "" || bindingID == "" {
		return nil, common.ErrThirdPartyBindingNotFound
	}
	raw, err := s.kv.Get(ctx, kv.KeyOAuthPendingBinding(bindingID))
	if err != nil {
		return nil, common.ErrThirdPartyBindingNotFound
	}
	var pending pendingThirdPartyBinding
	if err := json.Unmarshal([]byte(raw), &pending); err != nil || pending.UserID != userID {
		return nil, common.ErrThirdPartyBindingNotFound
	}
	return &pending, nil
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
	user, err := s.validateThirdPartyBinding(ctx, userID, profile)
	if err != nil {
		return user, err
	}

	if err := s.createThirdPartyBinding(ctx, userID, profile.Provider, profile.ProviderUID); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *OAuthService) validateThirdPartyBinding(ctx context.Context, userID string, profile *thirdPartyProfile) (*model.User, error) {
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
