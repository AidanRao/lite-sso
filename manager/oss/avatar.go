// Package oss manages objects stored in Alibaba Cloud OSS.
package oss

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	aliyunoss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/google/uuid"

	"sso-server/conf"
)

const avatarCacheControl = "public, max-age=31536000, immutable"

// AvatarStore stores and removes uploaded user avatars.
type AvatarStore interface {
	UploadAvatar(ctx context.Context, contentType string, extension string, body io.Reader, size int64) (string, string, error)
	DeleteAvatar(ctx context.Context, objectKey string) error
}

type objectClient interface {
	PutObject(ctx context.Context, request *aliyunoss.PutObjectRequest, optFns ...func(*aliyunoss.Options)) (*aliyunoss.PutObjectResult, error)
	DeleteObject(ctx context.Context, request *aliyunoss.DeleteObjectRequest, optFns ...func(*aliyunoss.Options)) (*aliyunoss.DeleteObjectResult, error)
}

// AvatarStorage stores avatar objects using a configured Alibaba Cloud OSS bucket.
type AvatarStorage struct {
	bucket        string
	avatarPrefix  string
	publicBaseURL *url.URL
	client        objectClient
}

// NewAvatarStorage builds an avatar store from OSS configuration.
func NewAvatarStorage(cfg conf.OSSConfig) (*AvatarStorage, error) {
	publicBaseURL, err := parsePublicBaseURL(cfg.PublicBaseURL)
	if err != nil {
		return nil, err
	}

	clientConfig := aliyunoss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.AccessKeySecret)).
		WithRegion(cfg.Region)
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		clientConfig.WithEndpoint(endpoint)
	}

	return newAvatarStorage(cfg, aliyunoss.NewClient(clientConfig), publicBaseURL)
}

func newAvatarStorage(cfg conf.OSSConfig, client objectClient, publicBaseURL *url.URL) (*AvatarStorage, error) {
	prefix := strings.Trim(strings.TrimSpace(cfg.AvatarPrefix), "/")
	if prefix == "" {
		return nil, fmt.Errorf("avatar prefix is required")
	}
	if client == nil {
		return nil, fmt.Errorf("OSS client is required")
	}
	if publicBaseURL == nil {
		return nil, fmt.Errorf("public base URL is required")
	}

	return &AvatarStorage{
		bucket:        strings.TrimSpace(cfg.Bucket),
		avatarPrefix:  prefix,
		publicBaseURL: publicBaseURL,
		client:        client,
	}, nil
}

// UploadAvatar uploads an avatar and returns its object key and public URL.
func (s *AvatarStorage) UploadAvatar(ctx context.Context, contentType string, extension string, body io.Reader, size int64) (string, string, error) {
	if body == nil {
		return "", "", fmt.Errorf("avatar body is required")
	}

	objectKey := path.Join(s.avatarPrefix, uuid.NewString()+extension)
	_, err := s.client.PutObject(ctx, &aliyunoss.PutObjectRequest{
		Bucket:        aliyunoss.Ptr(s.bucket),
		Key:           aliyunoss.Ptr(objectKey),
		CacheControl:  aliyunoss.Ptr(avatarCacheControl),
		ContentLength: aliyunoss.Ptr(size),
		ContentType:   aliyunoss.Ptr(contentType),
		Acl:           aliyunoss.ObjectACLPublicRead,
		Body:          body,
	})
	if err != nil {
		return "", "", err
	}

	publicURL := *s.publicBaseURL
	publicURL.Path = path.Join(publicURL.Path, objectKey)
	return objectKey, publicURL.String(), nil
}

// DeleteAvatar removes an avatar object by its internal key.
func (s *AvatarStorage) DeleteAvatar(ctx context.Context, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &aliyunoss.DeleteObjectRequest{
		Bucket: aliyunoss.Ptr(s.bucket),
		Key:    aliyunoss.Ptr(objectKey),
	})
	return err
}

func parsePublicBaseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid OSS public base URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("OSS public base URL must use HTTP or HTTPS")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}
