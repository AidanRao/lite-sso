// Package notification translates business events into user notifications.
package notification

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"sso-server/common/event"
	"sso-server/dal/db"
	"sso-server/manager/thirdparty"
	"sso-server/util/requestmeta"
)

const newDeviceLoginTemplateKey = "new_device_login"

// MessageSender submits messages to the delivery boundary.
type MessageSender interface {
	Send(context.Context, string, string, map[string]string) error
}

// LoginNotifier sends new-device notices to the current verified primary email.
type LoginNotifier struct {
	database    *gorm.DB
	sender      MessageSender
	securityURL string
	registry    *thirdparty.Registry
}

// NewLoginNotifier uses the configured public origin, never a request Host.
func NewLoginNotifier(database *gorm.DB, sender MessageSender, publicOrigin string, registry *thirdparty.Registry) *LoginNotifier {
	return &LoginNotifier{database: database, sender: sender, securityURL: strings.TrimRight(strings.TrimSpace(publicOrigin), "/") + "/profile/access/sessions", registry: registry}
}

// HandleLogin ignores existing devices and accounts without a verified primary email.
func (n *LoginNotifier) HandleLogin(ctx context.Context, fact event.Event) error {
	login, ok := fact.(event.LoginSucceeded)
	if !ok || !login.NewDevice || login.DeviceID == "" {
		return nil
	}
	user, err := db.NewUserRepository(n.database).FindByID(ctx, login.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("notification recipient lookup failed")
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return nil
	}
	recipient, err := db.NewUserEmailRepository(n.database).FindVerifiedPrimaryAt(ctx, login.UserID, login.OccurredAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("notification recipient lookup failed")
	}
	username := "用户"
	if user.Username != nil && strings.TrimSpace(*user.Username) != "" {
		username = strings.TrimSpace(*user.Username)
	}
	environment := requestmeta.ParseEnvironment(login.IP, login.UserAgent, login.Location)
	variables := map[string]string{
		"username":     username,
		"login_time":   login.OccurredAt.In(time.FixedZone("UTC+08:00", 8*60*60)).Format("2006-01-02 15:04:05") + "（UTC+08:00）",
		"device":       environment.DisplayName,
		"ip":           displayValue(environment.IP),
		"location":     displayValue(environment.Location),
		"auth_method":  n.authMethod(login.AuthMethod),
		"security_url": n.securityURL,
	}
	if err := n.sender.Send(ctx, recipient.Email, newDeviceLoginTemplateKey, variables); err != nil {
		return errors.New("notification submission failed")
	}
	return nil
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return value
}

func (n *LoginNotifier) authMethod(method string) string {
	switch method {
	case "PASSWORD":
		return "密码"
	case "EMAIL_OTP":
		return "邮箱验证码"
	case "QR":
		return "扫码"
	case "PASSKEY":
		return "通行密钥"
	case "GITHUB":
		return "GitHub"
	case "FEISHU":
		return "飞书"
	case "QQ":
		return "QQ"
	case "GOOGLE":
		return "Google"
	}
	for _, name := range n.registry.Names() {
		if registeredMethod, _ := n.registry.AuthMethod(name); registeredMethod == method {
			return name
		}
	}
	return "其他登录方式"
}
