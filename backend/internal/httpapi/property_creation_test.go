package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"
	"xiaopi/backend/internal/domain"
)

type creationReceiptStore struct {
	*fakeStore
	task, digest string
	saved        domain.Property
}

func (s *creationReceiptStore) CreatePropertyOnce(_ context.Context, p domain.Property, task, digest string) (domain.Property, error) {
	if s.task != "" {
		if s.task != task || s.digest != digest {
			return p, domain.ErrPropertyEditConflict
		}
		return s.saved, nil
	}
	s.task = task
	s.digest = digest
	s.saved = p
	return p, nil
}
func TestCreationDigestIncludesFieldsButNotGeneratedIDOrTime(t *testing.T) {
	storage := &creationReceiptStore{fakeStore: &fakeStore{}}
	server := &Server{store: storage}
	ctx := withClaims(context.Background(), tokenClaims{Subject: "admin", Role: roleAdmin})
	p := domain.Property{ID: "first", Title: "小区甲", Community: "小区甲", RentMonthly: 1000, CreatedAt: time.Now()}
	if _, err := server.createPropertyWithTask(ctx, p, "stable-task"); err != nil {
		t.Fatal(err)
	}
	p.ID = "retry"
	p.CreatedAt = time.Now().Add(time.Second)
	got, err := server.createPropertyWithTask(ctx, p, "stable-task")
	if err != nil || got.ID != "first" {
		t.Fatalf("same request did not replay: %v %v", got, err)
	}
	p.RentMonthly = 1200
	if _, err := server.createPropertyWithTask(ctx, p, "stable-task"); !errors.Is(err, domain.ErrPropertyEditConflict) {
		t.Fatalf("changed rent silently accepted: %v", err)
	}
}
