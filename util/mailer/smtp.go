package mailer

import (
	"context"
	"fmt"

	"github.com/go-mail/mail"
)

type SMTPConfig struct {
	Addr string // SMTP server address, e.g. "smtp.example.com:587"
	User string // SMTP username
	Pass string // SMTP password
	From string // From email address
}

type SMTPMailer struct {
	dialer *mail.Dialer
	from   string
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	// Extract host and port from address
	host, port := parseAddr(cfg.Addr)

	return &SMTPMailer{
		dialer: mail.NewDialer(host, port, cfg.User, cfg.Pass),
		from:   cfg.From,
	}
}

func (m *SMTPMailer) SendOTP(ctx context.Context, email string, otp string) error {
	if m.from == "" {
		return ErrNotConfigured
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", email)
	msg.SetHeader("Subject", "Your verification code")

	// Plain text version
	msg.SetBody("text/plain", fmt.Sprintf("Your verification code is: %s", otp))

	// HTML version
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .code {
            font-size: 32px;
            font-weight: bold;
            color: #0891B2;
            background: #ECFEFF;
            padding: 20px;
            text-align: center;
            border-radius: 8px;
            letter-spacing: 4px;
            margin: 20px 0;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #E2E8F0;
            font-size: 14px;
            color: #64748B;
        }
    </style>
</head>
<body>
    <div class="container">
        <h2>Your Verification Code</h2>
        <p>Use the following code to complete your verification:</p>
        <div class="code">%s</div>
        <p><strong>This code will expire in 5 minutes.</strong></p>
        <p>If you didn't request this code, please ignore this email.</p>
        <div class="footer">
            <p>Lite SSO - Secure Authentication Platform</p>
        </div>
    </div>
</body>
</html>
	`, otp)
	msg.AddAlternative("text/html", htmlBody)

	return m.dialer.DialAndSend(msg)
}

// parseAddr extracts host and port from address string
// Examples:
//   - "smtp.example.com:587" -> ("smtp.example.com", 587)
//   - "smtp.example.com" -> ("smtp.example.com", 25)
func parseAddr(addr string) (host string, port int) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host = addr[:i]
			fmt.Sscanf(addr[i+1:], "%d", &port)
			return
		}
	}
	host = addr
	port = 25 // default SMTP port
	return
}
