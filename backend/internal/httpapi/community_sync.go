package httpapi

import (
	"context"
	"log"
	"time"
	"xiaopi/backend/internal/store"
)

// Jobs are committed with the listing. Failed catalogue writes survive a
// cancelled request or a server restart and can be retried without user input.
func (s *Server) RunCommunitySync(ctx context.Context) {
	storage, ok := s.store.(interface {
		PendingCommunitySync(context.Context) ([]store.CommunitySyncJob, error)
		FinishCommunitySync(context.Context, uint64, string) error
	})
	if !ok {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		query, cancel := context.WithTimeout(ctx, 5*time.Second)
		jobs, err := storage.PendingCommunitySync(query)
		cancel()
		if err != nil {
			log.Printf("read community sync: %v", err)
		}
		for _, job := range jobs {
			work, stop := context.WithTimeout(ctx, 15*time.Second)
			p := job.Property
			err := s.ensureDoorplateCollectionPlan(work, p.Community, communityLocation{Address: p.Location, Latitude: p.Latitude, Longitude: p.Longitude})
			failure := ""
			if err != nil {
				failure = shortText(err.Error(), 500)
			}
			stop()
			finish, done := context.WithTimeout(ctx, 5*time.Second)
			if err := storage.FinishCommunitySync(finish, job.ID, failure); err != nil {
				log.Printf("finish community sync: %v", err)
			}
			done()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
