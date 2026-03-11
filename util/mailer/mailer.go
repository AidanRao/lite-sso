package mailer

import "context"

type Mailer interface {
	SendOTP(ctx context.Context, email string, otp string) error
}
