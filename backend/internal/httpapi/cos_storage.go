package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
)

func (s *Server) cosClient() (*cos.Client, error) {
	if !s.config.cosConfigured() {
		return nil, fmt.Errorf("COS is not configured")
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", s.config.COSBucket, s.config.COSRegion))
	if err != nil {
		return nil, err
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{Transport: &cos.AuthorizationTransport{
		SecretID:  s.config.COSSecretID,
		SecretKey: s.config.COSSecretKey,
	}}), nil
}

func (s *Server) putCOSObject(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	client, err := s.cosClient()
	if err != nil {
		return "", err
	}
	_, err = client.Object.Put(ctx, key, body, &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}})
	if err != nil {
		return "", err
	}
	return s.config.COSPublicBase + "/" + strings.TrimLeft(key, "/"), nil
}

func (s *Server) deleteCOSObject(ctx context.Context, objectURL string) {
	if !s.config.cosConfigured() || !strings.HasPrefix(objectURL, s.config.COSPublicBase+"/") {
		return
	}
	key := strings.TrimPrefix(objectURL, s.config.COSPublicBase+"/")
	if key == "" || path.Clean(key) != key || strings.HasPrefix(key, "../") {
		return
	}
	client, err := s.cosClient()
	if err == nil {
		_, _ = client.Object.Delete(ctx, key)
	}
}
