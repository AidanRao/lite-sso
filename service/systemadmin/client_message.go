package systemadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/manager/messagecenter"
)

const maximumMessageVariablesSize = 64 * 1024

var (
	ErrClientMessageInvalid           = errors.New("invalid client message")
	ErrClientMessageRecipientNotFound = errors.New("client message recipient not found")
	ErrClientMessageEmailRequired     = errors.New("client message recipient requires a verified primary email")
	ErrClientMessageUnavailable       = errors.New("client message sending unavailable")
)

// EmailMessageSender is the delivery boundary shared with the configured Message Center client.
type EmailMessageSender interface {
	SendEmail(context.Context, messagecenter.EmailMessage) error
}

// ClientMessageService resolves recipients without storing message content or jobs.
type ClientMessageService struct {
	database *gorm.DB
	sender   EmailMessageSender
}

// NewClientMessageService creates a client-scoped message submission service.
func NewClientMessageService(database *gorm.DB, sender EmailMessageSender) *ClientMessageService {
	return &ClientMessageService{database: database, sender: sender}
}

// Send revalidates membership and uses the current verified primary email on every submission.
func (s *ClientMessageService) Send(ctx context.Context, clientID uint, userID, key string, request dto.ClientMessageRequest) error {
	message, err := prepareClientMessage(request, key)
	if err != nil {
		return err
	}
	if s.sender == nil {
		return ErrClientMessageUnavailable
	}
	client, err := db.NewOAuthClientRepository(s.database).FindByID(ctx, clientID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrClientMessageRecipientNotFound
	}
	if err != nil {
		return err
	}
	if _, err := db.NewUserOAuthClientRepository(s.database).FindAssociation(ctx, userID, client.ClientID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrClientMessageRecipientNotFound
		}
		return err
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrClientMessageRecipientNotFound
	}
	if err != nil {
		return err
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return ErrClientMessageEmailRequired
	}
	message.Target = *user.Email
	message.IdempotencyKey = fmt.Sprintf("sso-admin:%d:%s:%s", clientID, user.ID, message.IdempotencyKey)
	return s.sender.SendEmail(ctx, message)
}

func prepareClientMessage(request dto.ClientMessageRequest, key string) (messagecenter.EmailMessage, error) {
	message := messagecenter.EmailMessage{SenderKey: strings.TrimSpace(request.SenderKey), TemplateKey: strings.TrimSpace(request.TemplateKey)}
	parsedKey, err := uuid.Parse(key)
	if err != nil || parsedKey == uuid.Nil || message.SenderKey == "" || message.TemplateKey == "" || utf8.RuneCountInString(message.SenderKey) > 100 || utf8.RuneCountInString(message.TemplateKey) > 100 {
		return message, ErrClientMessageInvalid
	}
	if err := json.Unmarshal(request.Variables, &message.Variables); err != nil || message.Variables == nil {
		return message, ErrClientMessageInvalid
	}
	encoded, err := json.Marshal(message.Variables)
	if err != nil || len(encoded) > maximumMessageVariablesSize {
		return message, ErrClientMessageInvalid
	}
	message.IdempotencyKey = parsedKey.String()
	return message, nil
}
