package captcha

import (
	"image/color"

	"github.com/mojocn/base64Captcha"
)

type Service struct {
	captcha *base64Captcha.Captcha
}

func NewService(store base64Captcha.Store) *Service {
	driver := base64Captcha.NewDriverString(
		80,
		240,
		4,
		base64Captcha.OptionShowHollowLine,
		4,
		"1234567890",
		&color.RGBA{R: 254, G: 254, B: 254, A: 254},
		base64Captcha.DefaultEmbeddedFonts,
		[]string{},
	)

	cp := base64Captcha.NewCaptcha(driver, store)
	return &Service{captcha: cp}
}

func (s *Service) Generate() (string, string, error) {
	id, b64, _, err := s.captcha.Generate()
	return id, b64, err
}
