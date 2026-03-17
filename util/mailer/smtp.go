package mailer

import (
	"context"

	"github.com/go-mail/mail"
)

type SMTPConfig struct {
	Host string // SMTP server host
	Port int    // SMTP server port
	User string // SMTP username
	Pass string // SMTP password
	From string // From email address
}

type SMTPMailer struct {
	dialer *mail.Dialer
	from   string
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	dialer := mail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Pass)
	dialer.SSL = true

	return &SMTPMailer{
		dialer: dialer,
		from:   cfg.From,
	}
}

func (m *SMTPMailer) SendEmail(ctx context.Context, email string, subject string, textBody string, htmlBody string) error {
	if m.from == "" {
		return ErrNotConfigured
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", email)
	msg.SetHeader("Subject", subject)

	// Plain text version
	msg.SetBody("text/plain", textBody)

	// HTML version if provided
	if htmlBody != "" {
		msg.AddAlternative("text/html", htmlBody)
	}

	return m.dialer.DialAndSend(msg)
}
