package oss

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	aliyunoss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"

	"sso-server/conf"
)

type fakeObjectClient struct {
	putRequest    *aliyunoss.PutObjectRequest
	putBody       []byte
	deleteRequest *aliyunoss.DeleteObjectRequest
	putErr        error
	deleteErr     error
}

func (c *fakeObjectClient) PutObject(_ context.Context, request *aliyunoss.PutObjectRequest, _ ...func(*aliyunoss.Options)) (*aliyunoss.PutObjectResult, error) {
	c.putRequest = request
	if request.Body != nil {
		var err error
		c.putBody, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
	}
	return &aliyunoss.PutObjectResult{}, c.putErr
}

func (c *fakeObjectClient) DeleteObject(_ context.Context, request *aliyunoss.DeleteObjectRequest, _ ...func(*aliyunoss.Options)) (*aliyunoss.DeleteObjectResult, error) {
	c.deleteRequest = request
	return &aliyunoss.DeleteObjectResult{}, c.deleteErr
}

func TestAvatarStorage_UploadImage(t *testing.T) {
	client := &fakeObjectClient{}
	publicBaseURL, err := parsePublicBaseURL("https://cdn.example.com/profile-images/")
	if err != nil {
		t.Fatalf("parse public base URL: %v", err)
	}
	storage, err := newAvatarStorage(conf.OSSConfig{
		Bucket:       "avatars",
		AvatarPrefix: "sso/user-assets",
	}, client, publicBaseURL)
	if err != nil {
		t.Fatalf("new avatar storage: %v", err)
	}

	objectKey, publicURL, err := storage.UploadImage(context.Background(), "image/png", ".png", bytes.NewBufferString("image-data"), 10)
	if err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	if !strings.HasPrefix(objectKey, "sso/user-assets/") || !strings.HasSuffix(objectKey, ".png") {
		t.Fatalf("unexpected object key: %s", objectKey)
	}
	if publicURL != "https://cdn.example.com/profile-images/"+objectKey {
		t.Fatalf("unexpected public URL: %s", publicURL)
	}
	if client.putRequest == nil || *client.putRequest.Bucket != "avatars" || *client.putRequest.Key != objectKey {
		t.Fatalf("unexpected upload request: %#v", client.putRequest)
	}
	if client.putRequest.Acl != aliyunoss.ObjectACLPublicRead || *client.putRequest.ContentType != "image/png" {
		t.Fatalf("unexpected ACL or content type: %#v", client.putRequest)
	}
	if *client.putRequest.ContentLength != 10 || string(client.putBody) != "image-data" {
		t.Fatalf("unexpected upload body: size=%d body=%q", *client.putRequest.ContentLength, client.putBody)
	}
}

func TestAvatarStorage_DeleteImage(t *testing.T) {
	client := &fakeObjectClient{}
	publicBaseURL, err := parsePublicBaseURL("https://cdn.example.com")
	if err != nil {
		t.Fatalf("parse public base URL: %v", err)
	}
	storage, err := newAvatarStorage(conf.OSSConfig{Bucket: "avatars", AvatarPrefix: "avatars"}, client, publicBaseURL)
	if err != nil {
		t.Fatalf("new avatar storage: %v", err)
	}

	if err := storage.DeleteImage(context.Background(), "sso/user-assets/old.png"); err != nil {
		t.Fatalf("delete avatar: %v", err)
	}
	if client.deleteRequest == nil || *client.deleteRequest.Key != "sso/user-assets/old.png" {
		t.Fatalf("unexpected delete request: %#v", client.deleteRequest)
	}
}

func TestAvatarStorage_UploadAvatarReturnsStorageError(t *testing.T) {
	client := &fakeObjectClient{putErr: errors.New("OSS unavailable")}
	publicBaseURL, err := parsePublicBaseURL("https://cdn.example.com")
	if err != nil {
		t.Fatalf("parse public base URL: %v", err)
	}
	storage, err := newAvatarStorage(conf.OSSConfig{Bucket: "avatars", AvatarPrefix: "avatars"}, client, publicBaseURL)
	if err != nil {
		t.Fatalf("new avatar storage: %v", err)
	}

	if _, _, err := storage.UploadImage(context.Background(), "image/png", ".png", bytes.NewBufferString("image-data"), 10); err == nil {
		t.Fatal("expected OSS upload error")
	}
}

func TestParsePublicBaseURL_RejectsInvalidURL(t *testing.T) {
	if _, err := parsePublicBaseURL("cdn.example.com"); err == nil {
		t.Fatal("expected invalid public base URL error")
	}
}
