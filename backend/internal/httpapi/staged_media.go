package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"xiaopi/backend/internal/domain"
)

type stagedMediaStore interface {
	ExpiredStagedMedia(context.Context, time.Time, int) ([]domain.PropertyMedia, error)
	RemoveExpiredStagedMedia(context.Context, domain.PropertyMedia, time.Time, func(context.Context, string) error) error
}

func (s *Server) RunStagedMediaCleanup(ctx context.Context) {
	store, ok := s.store.(stagedMediaStore)
	if !ok {
		return
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		s.cleanupStagedMedia(ctx, store)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) cleanupStagedMedia(ctx context.Context, store stagedMediaStore) {
	cutoff := time.Now().Add(-48 * time.Hour)
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	items, err := store.ExpiredStagedMedia(queryCtx, cutoff, 100)
	cancel()
	if err != nil {
		log.Printf("read expired staged media: %v", err)
		return
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		itemCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := store.RemoveExpiredStagedMedia(itemCtx, item, cutoff, s.removeExpiredUpload)
		cancel()
		if err != nil {
			log.Printf("clean expired staged media %s: %v", item.ID, err)
		}
	}
}

func (s *Server) removeExpiredUpload(ctx context.Context, objectURL string) error {
	if name, local := strings.CutPrefix(objectURL, "/uploads/"); local {
		if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
			return fmt.Errorf("invalid staged local upload name")
		}
		err := os.Remove(filepath.Join(mediaStorageDir(), name))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if key, ok := strings.CutPrefix(objectURL, s.config.COSPublicBase+"/"); s.config.cosConfigured() && ok && strings.HasPrefix(key, "media/") && path.Clean(key) == key {
		return s.deleteCOSObject(ctx, objectURL)
	}
	return fmt.Errorf("staged upload is outside configured media storage")
}
