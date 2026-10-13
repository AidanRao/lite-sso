package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dal/kv"
)

const (
	qrCodeExpire = 3 * time.Minute
	// QRLoginScope allows an application to approve browser SSO login.
	QRLoginScope = "qr:login"
	// QRLoginAudience identifies the API accepting scanner OAuth tokens.
	QRLoginAudience = "sso-api"
)

//go:embed qr_transition.lua
var qrTransitionScript embed.FS
var qrFallbackMu sync.Mutex

// QRCodeStatus identifies a QR login request's state.
type QRCodeStatus string

const (
	QRCodeStatusPending   QRCodeStatus = "pending"
	QRCodeStatusScanned   QRCodeStatus = "scanned"
	QRCodeStatusConfirmed QRCodeStatus = "confirmed"
	QRCodeStatusCancelled QRCodeStatus = "cancelled"
	QRCodeStatusConsumed  QRCodeStatus = "consumed"
)

// QRActor is the verified OAuth identity approving a browser login.
type QRActor struct {
	UserID   string `json:"user_id"`
	ClientID string `json:"client_id"`
}

// QRCodeData is private storage, never a public response.
type QRCodeData struct {
	Code          string       `json:"code"`
	Status        QRCodeStatus `json:"status"`
	UserID        string       `json:"user_id"`
	ClientID      string       `json:"client_id"`
	DeviceID      string       `json:"device_id"`
	BrowserDigest string       `json:"browser_digest"`
	LoginTicket   string       `json:"login_ticket"`
	Redirect      string       `json:"redirect"`
	TargetName    string       `json:"target_name"`
	IP            string       `json:"ip"`
	UserAgent     string       `json:"user_agent"`
	CreatedAt     time.Time    `json:"created_at"`
	ExpiresAt     time.Time    `json:"expires_at"`
}

// QRCodeChallenge separates the scannable payload from browser-only credentials.
type QRCodeChallenge struct {
	Code         string    `json:"code"`
	QRPayload    string    `json:"qr_payload"`
	BrowserToken string    `json:"browser_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// QRCodePreview contains the information shown before the user approves login.
type QRCodePreview struct {
	Status      QRCodeStatus `json:"status"`
	UserID      string       `json:"user_id"`
	AccountName string       `json:"account_name"`
	TargetName  string       `json:"target_name"`
	IP          string       `json:"ip"`
	UserAgent   string       `json:"user_agent"`
	CreatedAt   time.Time    `json:"created_at"`
	ExpiresAt   time.Time    `json:"expires_at"`
}

// GenerateQRCode creates a fixed-lifetime request bound to the requesting browser.
func (s *AuthService) GenerateQRCode(ctx context.Context, redirect string, metadata LoginMetadata) (*QRCodeChallenge, error) {
	redirect, err := NormalizeLoginRedirect(redirect)
	if err != nil {
		return nil, err
	}
	target := "SSO"
	parsed, _ := url.Parse(redirect)
	if parsed.Path == "/oauth/authorize" {
		client, lookupErr := db.NewOAuthClientRepository(s.db).FindByClientID(ctx, parsed.Query().Get("client_id"))
		if lookupErr != nil {
			return nil, common.ErrInvalidRedirect
		}
		target = client.Name
	}
	secret, err := generateSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	data := QRCodeData{Code: uuid.NewString(), Status: QRCodeStatusPending, DeviceID: metadata.DeviceID,
		BrowserDigest: qrDigest(secret), Redirect: redirect, TargetName: target, IP: metadata.IP,
		UserAgent: metadata.UserAgent, CreatedAt: now, ExpiresAt: now.Add(qrCodeExpire)}
	payload, err := json.Marshal(struct {
		Type    string `json:"type"`
		Version int    `json:"version"`
		Issuer  string `json:"issuer"`
		Code    string `json:"code"`
	}{Type: "lite-sso.qr-login", Version: 1, Issuer: s.cfg.Tokens.OAuth.Issuer, Code: data.Code})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := s.kv.Set(ctx, kv.KeyQR(data.Code), string(raw), qrCodeExpire); err != nil {
		return nil, err
	}
	return &QRCodeChallenge{Code: data.Code, QRPayload: string(payload), BrowserToken: secret, ExpiresAt: data.ExpiresAt}, nil
}

// ValidateQRCodeActor checks live application permissions and account status.
func (s *AuthService) ValidateQRCodeActor(ctx context.Context, actor QRActor) error {
	if actor.UserID == "" || actor.ClientID == "" {
		return common.ErrQRCodeClientForbidden
	}
	client, err := db.NewOAuthClientRepository(s.db).FindByClientID(ctx, actor.ClientID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrQRCodeClientForbidden
	}
	if err != nil {
		return err
	}
	if !slices.Contains(client.AllowedScopes, QRLoginScope) || !slices.Contains(client.Audiences, QRLoginAudience) {
		return common.ErrQRCodeClientForbidden
	}
	user, err := db.NewUserRepository(s.db).FindByID(ctx, actor.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if !user.IsActive {
		return common.ErrUserInactive
	}
	return nil
}

// PollQRCode only reveals status and tickets to the original browser.
func (s *AuthService) PollQRCode(ctx context.Context, code, deviceID, browserToken string) (*QRCodeData, error) {
	data, _, err := s.loadQRCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if err := data.validateBrowser(deviceID, browserToken); err != nil {
		return nil, err
	}
	return data, nil
}

// ScanQRCode binds the first scanner and returns a safe confirmation preview.
func (s *AuthService) ScanQRCode(ctx context.Context, code string, actor QRActor) (*QRCodePreview, error) {
	if err := s.ValidateQRCodeActor(ctx, actor); err != nil {
		return nil, err
	}
	data, err := s.transitionQRCode(ctx, code, "scan", actor, "", "", "")
	if err != nil {
		return nil, err
	}
	user, err := db.NewUserRepository(s.db).FindByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	name := user.ID
	if user.Username != nil && *user.Username != "" {
		name = *user.Username
	} else if user.Email != nil {
		name = *user.Email
	}
	return &QRCodePreview{Status: data.Status, UserID: user.ID, AccountName: name, TargetName: data.TargetName,
		IP: data.IP, UserAgent: data.UserAgent, CreatedAt: data.CreatedAt, ExpiresAt: data.ExpiresAt}, nil
}

// ConfirmQRCode issues one ticket and supports retries by the original scanner.
func (s *AuthService) ConfirmQRCode(ctx context.Context, code string, actor QRActor) error {
	if err := s.ValidateQRCodeActor(ctx, actor); err != nil {
		return err
	}
	ticket, err := generateSessionID()
	if err != nil {
		return err
	}
	_, err = s.transitionQRCode(ctx, code, "confirm", actor, "", "", ticket)
	return err
}

// RejectQRCode cancels the request on behalf of its original scanner.
func (s *AuthService) RejectQRCode(ctx context.Context, code string, actor QRActor) error {
	if err := s.ValidateQRCodeActor(ctx, actor); err != nil {
		return err
	}
	_, err := s.transitionQRCode(ctx, code, "reject", actor, "", "", "")
	return err
}

// CancelQRCode invalidates a request when the browser leaves or refreshes it.
func (s *AuthService) CancelQRCode(ctx context.Context, code, deviceID, browserToken string) error {
	_, err := s.transitionQRCode(ctx, code, "cancel", QRActor{}, deviceID, browserToken, "")
	return err
}

// CompleteQRCodeLogin consumes the request before creating exactly one browser session.
func (s *AuthService) CompleteQRCodeLogin(ctx context.Context, code, ticket, browserToken string, metadata LoginMetadata) (*LoginResult, *TokenPair, error) {
	data, err := s.PollQRCode(ctx, code, metadata.DeviceID, browserToken)
	if err != nil {
		return nil, nil, err
	}
	if data.Status != QRCodeStatusConfirmed {
		return nil, nil, common.ErrQRCodeInvalidStatus
	}
	if err := s.ValidateQRCodeActor(ctx, QRActor{UserID: data.UserID, ClientID: data.ClientID}); err != nil {
		return nil, nil, err
	}
	data, err = s.transitionQRCode(ctx, code, "complete", QRActor{}, metadata.DeviceID, browserToken, ticket)
	if err != nil {
		return nil, nil, err
	}
	return s.CompleteLoginWithContext(ctx, data.UserID, data.Redirect, metadata, AuthMethodQR)
}

func qrDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) loadQRCode(ctx context.Context, code string) (*QRCodeData, string, error) {
	raw, err := s.kv.Get(ctx, kv.KeyQR(code))
	if errors.Is(err, kv.ErrNotFound) {
		return nil, "", common.ErrQRCodeExpired
	}
	if err != nil {
		return nil, "", err
	}
	var data QRCodeData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, "", err
	}
	if !time.Now().Before(data.ExpiresAt) {
		return nil, "", common.ErrQRCodeExpired
	}
	return &data, raw, nil
}

// Both storage backends execute the same state machine. Redis uses compare-and-set
// so concurrent workers cannot overwrite a confirmation or consume it twice.
func (s *AuthService) transitionQRCode(ctx context.Context, code, operation string, actor QRActor, deviceID, browserToken, ticket string) (*QRCodeData, error) {
	qrFallbackMu.Lock()
	defer qrFallbackMu.Unlock()
	script, err := qrTransitionScript.ReadFile("qr_transition.lua")
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 10; attempt++ {
		data, raw, err := s.loadQRCode(ctx, code)
		if err != nil {
			return nil, err
		}
		if err := data.transition(operation, actor, deviceID, browserToken, ticket); err != nil {
			return nil, err
		}
		updated, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		result, err := s.security.Eval(ctx, string(script), []string{kv.KeyQR(code)}, raw, string(updated))
		if errors.Is(err, kv.ErrScriptUnsupported) {
			remaining := time.Until(data.ExpiresAt)
			if remaining <= 0 {
				return nil, common.ErrQRCodeExpired
			}
			if err := s.kv.Set(ctx, kv.KeyQR(code), string(updated), remaining); err != nil {
				return nil, err
			}
			return data, nil
		}
		if err != nil {
			return nil, err
		}
		switch fmt.Sprint(result) {
		case "1":
			return data, nil
		case "-1":
			return nil, common.ErrQRCodeExpired
		case "0":
			continue
		default:
			return nil, errors.New("invalid QR transition response")
		}
	}
	return nil, common.ErrQRCodeInvalidStatus
}

func (data *QRCodeData) validateBrowser(deviceID, token string) error {
	if deviceID == "" || deviceID != data.DeviceID || token == "" ||
		subtle.ConstantTimeCompare([]byte(qrDigest(token)), []byte(data.BrowserDigest)) != 1 {
		return common.ErrQRCodeBrowserInvalid
	}
	return nil
}

func (data *QRCodeData) transition(operation string, actor QRActor, deviceID, browserToken, ticket string) error {
	if operation == "cancel" || operation == "complete" {
		if err := data.validateBrowser(deviceID, browserToken); err != nil {
			return err
		}
	} else if data.Status != QRCodeStatusPending && (data.UserID != actor.UserID || data.ClientID != actor.ClientID) {
		return common.ErrQRCodeInvalidUser
	}
	switch operation {
	case "scan":
		if data.Status == QRCodeStatusPending {
			data.UserID, data.ClientID, data.Status = actor.UserID, actor.ClientID, QRCodeStatusScanned
		} else if data.Status != QRCodeStatusScanned && data.Status != QRCodeStatusConfirmed {
			return common.ErrQRCodeInvalidStatus
		}
	case "confirm":
		if data.Status == QRCodeStatusScanned {
			data.Status, data.LoginTicket = QRCodeStatusConfirmed, ticket
		} else if data.Status != QRCodeStatusConfirmed {
			return common.ErrQRCodeInvalidStatus
		}
	case "reject":
		if data.Status != QRCodeStatusScanned && data.Status != QRCodeStatusCancelled {
			return common.ErrQRCodeInvalidStatus
		}
		data.Status, data.LoginTicket = QRCodeStatusCancelled, ""
	case "cancel":
		if data.Status == QRCodeStatusConsumed {
			return common.ErrQRCodeInvalidStatus
		}
		data.Status, data.LoginTicket = QRCodeStatusCancelled, ""
	case "complete":
		if data.Status != QRCodeStatusConfirmed {
			return common.ErrQRCodeInvalidStatus
		}
		if ticket == "" || subtle.ConstantTimeCompare([]byte(ticket), []byte(data.LoginTicket)) != 1 {
			return common.ErrQRCodeInvalidTicket
		}
		data.Status, data.LoginTicket = QRCodeStatusConsumed, ""
	default:
		return common.ErrQRCodeInvalidStatus
	}
	return nil
}
