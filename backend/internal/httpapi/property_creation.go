package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
	"xiaopi/backend/internal/domain"
)

func (s *Server) createPropertyWithTask(ctx context.Context, p domain.Property, key string) (domain.Property, error) {
	if key == "" {
		return p, s.store.CreateProperty(ctx, p)
	}
	stable := p
	stable.ID = ""
	stable.CreatedAt = time.Time{}
	stable.UpdatedAt = time.Time{}
	stable.Media = nil
	stable.EditRevision = 0
	stable.ConvertedPropertyID = ""
	data, err := json.Marshal(stable)
	if err != nil {
		return p, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	task := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%t\x00%s", claimsFromContext(ctx).Subject, p.IsMaterial, key))))
	if storage, ok := s.store.(interface {
		CreatePropertyOnce(context.Context, domain.Property, string, string) (domain.Property, error)
	}); ok {
		return storage.CreatePropertyOnce(ctx, p, task, digest)
	}
	// Legacy test stores do not implement durable task receipts.
	return p, s.store.CreateProperty(ctx, p)
}
