package auth

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"sso-server/common"
	"sso-server/dal/kv"
)

const (
	qrCodeExpire = 5 * time.Minute
)

//go:embed qr_transition.lua
var qrTransitionScript embed.FS

var qrFallbackMu sync.Mutex

type QRCodeStatus string

const (
	QRCodeStatusPending   QRCodeStatus = "pending"
	QRCodeStatusScanned   QRCodeStatus = "scanned"
	QRCodeStatusConfirmed QRCodeStatus = "confirmed"
	QRCodeStatusExpired   QRCodeStatus = "expired"
)

type QRCodeData struct {
	Code        string       `json:"code"`
	Status      QRCodeStatus `json:"status"`
	UserID      string       `json:"user_id,omitempty"`
	DeviceID    string       `json:"device_id,omitempty"`
	LoginTicket string       `json:"login_ticket,omitempty"`
	Redirect    string       `json:"redirect,omitempty"`
}

// GenerateQRCode generates a new QR code for QR code login
func (s *AuthService) GenerateQRCode(ctx context.Context, redirect string) (string, error) {
	return s.GenerateQRCodeWithDevice(ctx, redirect, "")
}

func (s *AuthService) GenerateQRCodeWithDevice(ctx context.Context, redirect string, deviceID string) (string, error) {
	redirectURL, err := NormalizeLoginRedirect(redirect)
	if err != nil {
		return "", err
	}

	code := uuid.New().String()

	qrData := QRCodeData{
		Code:     code,
		Status:   QRCodeStatusPending,
		Redirect: redirectURL,
		DeviceID: deviceID,
	}

	data, err := json.Marshal(qrData)
	if err != nil {
		return "", err
	}

	if err := s.kv.Set(ctx, kv.KeyQR(code), string(data), qrCodeExpire); err != nil {
		return "", err
	}

	return code, nil
}

// PollQRCode polls the status of a QR code
func (s *AuthService) PollQRCode(ctx context.Context, code string) (*QRCodeData, error) {
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return nil, common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return nil, err
	}

	return &qrData, nil
}

// ScanQRCode marks a QR code as scanned by a user
func (s *AuthService) ScanQRCode(ctx context.Context, code, userID string) error {
	if result, err := s.evalQRTransition(ctx, code, "scan", userID, "", ""); err == nil {
		if result != "scanned" {
			return common.ErrQRCodeInvalidStatus
		}
		return nil
	} else if !errors.Is(err, kv.ErrScriptUnsupported) {
		return err
	}
	qrFallbackMu.Lock()
	defer qrFallbackMu.Unlock()
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return err
	}

	if qrData.Status != QRCodeStatusPending {
		return common.ErrQRCodeInvalidStatus
	}

	qrData.Status = QRCodeStatusScanned
	qrData.UserID = userID

	updated, err := json.Marshal(qrData)
	if err != nil {
		return err
	}

	return s.kv.Set(ctx, kv.KeyQR(code), string(updated), qrCodeExpire)
}

// ConfirmQRCode confirms a QR code login and creates a one-time browser login ticket.
func (s *AuthService) ConfirmQRCode(ctx context.Context, code, userID string) error {
	ticket, err := generateSessionID()
	if err != nil {
		return err
	}
	if result, transitionErr := s.evalQRTransition(ctx, code, "confirm", userID, ticket, ""); transitionErr == nil {
		if result != "confirmed" {
			return common.ErrQRCodeInvalidStatus
		}
		return nil
	} else if !errors.Is(transitionErr, kv.ErrScriptUnsupported) {
		return transitionErr
	}
	qrFallbackMu.Lock()
	defer qrFallbackMu.Unlock()
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return err
	}

	if qrData.Status != QRCodeStatusScanned {
		return common.ErrQRCodeInvalidStatus
	}

	if qrData.UserID != userID {
		return common.ErrQRCodeInvalidUser
	}

	qrData.Status = QRCodeStatusConfirmed
	qrData.LoginTicket = ticket
	updated, err := json.Marshal(qrData)
	if err != nil {
		return err
	}

	if err := s.kv.Set(ctx, kv.KeyQR(code), string(updated), qrCodeExpire); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) CompleteQRCodeLogin(ctx context.Context, code string, loginTicket string) (*LoginResult, string, error) {
	result, pair, err := s.CompleteQRCodeLoginWithDevice(ctx, code, loginTicket, "")
	if err != nil {
		return nil, "", err
	}
	claims, err := s.ParseAccessToken(result.AccessToken)
	if err != nil {
		return nil, "", err
	}
	_ = pair
	return result, claims.SessionID, nil
}

func (s *AuthService) CompleteQRCodeLoginWithDevice(ctx context.Context, code string, loginTicket string, deviceID string) (*LoginResult, *TokenPair, error) {
	return s.CompleteQRCodeLoginWithMetadata(ctx, code, loginTicket, LoginMetadata{DeviceID: deviceID})
}

func (s *AuthService) CompleteQRCodeLoginWithMetadata(ctx context.Context, code string, loginTicket string, metadata LoginMetadata) (*LoginResult, *TokenPair, error) {
	if result, err := s.evalQRCompletion(ctx, code, loginTicket, metadata.DeviceID); err == nil {
		userID, redirect := result[0], result[1]
		loginResult, pair, completeErr := s.CompleteLoginWithContext(ctx, userID, redirect, metadata, AuthMethodQR)
		if completeErr != nil {
			return nil, nil, completeErr
		}
		return loginResult, pair, nil
	} else if !errors.Is(err, kv.ErrScriptUnsupported) {
		return nil, nil, err
	}

	qrFallbackMu.Lock()
	defer qrFallbackMu.Unlock()
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return nil, nil, common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return nil, nil, err
	}

	if qrData.Status != QRCodeStatusConfirmed {
		return nil, nil, common.ErrQRCodeInvalidStatus
	}

	if qrData.LoginTicket == "" || qrData.LoginTicket != loginTicket {
		return nil, nil, common.ErrQRCodeInvalidTicket
	}
	if qrData.DeviceID != "" && qrData.DeviceID != metadata.DeviceID {
		return nil, nil, common.ErrChallengeInvalid
	}

	result, pair, err := s.CompleteLoginWithContext(ctx, qrData.UserID, qrData.Redirect, metadata, AuthMethodQR)
	if err != nil {
		return nil, nil, err
	}

	_ = s.kv.Del(ctx, kv.KeyQR(code))

	return result, pair, nil
}

func (s *AuthService) evalQRTransition(ctx context.Context, code string, operation string, userID string, ticket string, deviceID string) (string, error) {
	script, err := qrTransitionScript.ReadFile("qr_transition.lua")
	if err != nil {
		return "", err
	}
	value, err := s.security.Eval(ctx, string(script), []string{kv.KeyQR(code)}, operation, userID, ticket, deviceID)
	if err != nil {
		return "", err
	}
	values, ok := value.([]interface{})
	if !ok || len(values) < 2 || fmt.Sprint(values[0]) != "1" {
		return "", common.ErrQRCodeInvalidStatus
	}
	return fmt.Sprint(values[1]), nil
}

func (s *AuthService) evalQRCompletion(ctx context.Context, code string, ticket string, deviceID string) ([]string, error) {
	script, err := qrTransitionScript.ReadFile("qr_transition.lua")
	if err != nil {
		return nil, err
	}
	value, err := s.security.Eval(ctx, string(script), []string{kv.KeyQR(code)}, "complete", ticket, deviceID)
	if err != nil {
		return nil, err
	}
	values, ok := value.([]interface{})
	if !ok || len(values) < 3 || fmt.Sprint(values[0]) != "1" {
		return nil, common.ErrQRCodeInvalidTicket
	}
	return []string{fmt.Sprint(values[1]), fmt.Sprint(values[2])}, nil
}
