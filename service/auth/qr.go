package auth

import (
	"context"
	"encoding/json"
	"net/http"
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
	QRCodeStatusPending  QRCodeStatus = "pending"
	QRCodeStatusScanned  QRCodeStatus = "scanned"
	QRCodeStatusConfirmed QRCodeStatus = "confirmed"
	QRCodeStatusExpired   QRCodeStatus = "expired"
)

type QRCodeData struct {
	Code   string       `json:"code"`
	Status QRCodeStatus `json:"status"`
	UserID string       `json:"user_id,omitempty"`
}

// GenerateQRCode generates a new QR code for QR code login
func (s *AuthService) GenerateQRCode(ctx context.Context) (string, error) {
	code := uuid.New().String()

	qrData := QRCodeData{
		Code:   code,
		Status: QRCodeStatusPending,
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

// ConfirmQRCode confirms a QR code login and issues a token
func (s *AuthService) ConfirmQRCode(ctx context.Context, r *http.Request, code, userID string) (map[string]interface{}, error) {
	data, err := s.kv.Get(ctx, kv.KeyQR(code))
	if err != nil {
		return nil, common.ErrQRCodeExpired
	}

	var qrData QRCodeData
	if err := json.Unmarshal([]byte(data), &qrData); err != nil {
		return nil, err
	}

	if qrData.Status != QRCodeStatusScanned {
		return nil, common.ErrQRCodeInvalidStatus
	}

	if qrData.UserID != userID {
		return nil, common.ErrQRCodeInvalidUser
	}

	qrData.Status = QRCodeStatusConfirmed
	updated, err := json.Marshal(qrData)
	if err != nil {
		return nil, err
	}

	if err := s.kv.Set(ctx, kv.KeyQR(code), string(updated), qrCodeExpire); err != nil {
		return nil, err
	}

	if s.oauth2 == nil || r == nil {
		return nil, nil
	}

	tokenData, err := s.oauth2.IssueTokenForUser(ctx, r, userID)
	if err != nil {
		return nil, err
	}

	return tokenData, nil
}
