package mailer

import "context"

type Mailer interface {
	SendEmail(ctx context.Context, email string, subject string, textBody string, htmlBody string) error
}
