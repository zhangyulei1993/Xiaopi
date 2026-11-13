package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
)

func (s *Server) cosClient() (*cos.Client, error) {
	if os.Getenv("APP_ENV") == "test" {
		return nil, fmt.Errorf("COS is disabled in test mode")
	}
	if !s.config.cosConfigured() {
		return nil, fmt.Errorf("COS is not configured")
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", s.config.COSBucket, s.config.COSRegion))
	if err != nil {
		return nil, err
	}
	policyTransport := newOutboundPolicyTransport(http.DefaultTransport, os.Getenv("APP_ENV") == "test", testOutboundTarget())
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{Transport: &cos.AuthorizationTransport{
		SecretID:  s.config.COSSecretID,
		SecretKey: s.config.COSSecretKey,
		Transport: policyTransport,
	}}), nil
}

func (s *Server) putCOSObject(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	if os.Getenv("APP_ENV") == "test" {
		return "", fmt.Errorf("COS upload is disabled in test mode")
	}
	client, err := s.cosClient()
	if err != nil {
		return "", err
	}
	_, err = client.Object.Put(ctx, key, body, &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}, ACLHeaderOptions: &cos.ACLHeaderOptions{XCosACL: "public-read"}})
	if err != nil {
		return "", err
	}
	return s.config.COSPublicBase + "/" + strings.TrimLeft(key, "/"), nil
}

func (s *Server) deleteCOSObject(ctx context.Context, objectURL string) error {
	if os.Getenv("APP_ENV") == "test" {
		return fmt.Errorf("COS delete is disabled in test mode")
	}
	if !s.config.cosConfigured() || !strings.HasPrefix(objectURL, s.config.COSPublicBase+"/") {
		return nil
	}
	key := strings.TrimPrefix(objectURL, s.config.COSPublicBase+"/")
	if key == "" || path.Clean(key) != key || strings.HasPrefix(key, "../") {
		return nil
	}
	client, err := s.cosClient()
	if err == nil {
		_, err = client.Object.Delete(ctx, key)
	}
	return err
}
