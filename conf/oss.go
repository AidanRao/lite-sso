package conf

import (
	"errors"
	"strings"
)

// OSSConfig contains the Alibaba Cloud OSS settings used for user avatars.
type OSSConfig struct {
	Region          string `mapstructure:"region"`
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	AvatarPrefix    string `mapstructure:"avatar_prefix"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
}

// IsConfigured reports whether any OSS setting has been provided.
func (c OSSConfig) IsConfigured() bool {
	return strings.TrimSpace(c.Region) != "" ||
		strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Bucket) != "" ||
		strings.TrimSpace(c.AccessKeyID) != "" ||
		strings.TrimSpace(c.AccessKeySecret) != "" ||
		strings.TrimSpace(c.AvatarPrefix) != "" ||
		strings.TrimSpace(c.PublicBaseURL) != ""
}

// Validate checks that all required OSS settings are supplied together.
func (c OSSConfig) Validate() error {
	if !c.IsConfigured() {
		return nil
	}

	values := map[string]string{
		"oss.region":            c.Region,
		"oss.bucket":            c.Bucket,
		"oss.access_key_id":     c.AccessKeyID,
		"oss.access_key_secret": c.AccessKeySecret,
		"oss.avatar_prefix":     c.AvatarPrefix,
		"oss.public_base_url":   c.PublicBaseURL,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + " is required when OSS is configured")
		}
	}
	return nil
}
