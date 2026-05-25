package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"sso-server/common"
	"sso-server/dal/kv"
)

const (
	qrCodeExpire = 5 * time.Minute
)

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
	LoginTicket string       `json:"login_ticket,omitempty"`
	Redirect    string       `json:"redirect,omitempty"`
}

// GenerateQRCode generates a new QR code for QR code login
func (s *AuthService) GenerateQRCode(ctx context.Context, redirect string) (string, error) {
	redirectURL, err := NormalizeLoginRedirect(redirect)
	if err != nil {
		return "", err
	}

	code := uuid.New().String()

	qrData := QRCodeData{
		Code:     code,
		Status:   QRCodeStatusPending,
		Redirect: redirectURL,
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

	ticket, err := generateSessionID()
	if err != nil {
		return err
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
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return nil, "", common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return nil, "", err
	}

	if qrData.Status != QRCodeStatusConfirmed {
		return nil, "", common.ErrQRCodeInvalidStatus
	}

	if qrData.LoginTicket == "" || qrData.LoginTicket != loginTicket {
		return nil, "", common.ErrQRCodeInvalidTicket
	}

	result, sessionID, err := s.CompleteLogin(ctx, qrData.UserID, qrData.Redirect)
	if err != nil {
		return nil, "", err
	}

	_ = s.kv.Del(ctx, kv.KeyQR(code))

	return result, sessionID, nil
}
