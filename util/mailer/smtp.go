package mailer

import (
	"context"
	"fmt"
	"net/smtp"
)

type SMTPConfig struct {
	Addr string
	User string
	Pass string
	From string
}

type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) SendOTP(ctx context.Context, email string, otp string) error {
	if m.cfg.Addr == "" || m.cfg.From == "" {
		return ErrNotConfigured
	}

	auth := smtp.Auth(nil)
	if m.cfg.User != "" || m.cfg.Pass != "" {
		host := m.cfg.Addr
		if i := indexHost(host); i >= 0 {
			host = host[:i]
		}
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, host)
	}

	subject := "Your verification code"
	body := fmt.Sprintf("Your verification code is: %s\n", otp)
	msg := []byte("To: " + email + "\r\n" +
		"From: " + m.cfg.From + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" + body)

	return smtp.SendMail(m.cfg.Addr, auth, m.cfg.From, []string{email}, msg)
}

func indexHost(addr string) int {
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			return i
		}
	}
	return -1
}
