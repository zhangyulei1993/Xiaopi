package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"xiaopi/backend/internal/domain"
)

var errSubscriptionBatch = errors.New("通知接收人分批处理中")

type subscriptionOutboxStore interface {
	PendingSubscriptionJobs(context.Context) ([]domain.SubscriptionJob, error)
	FinishSubscriptionJob(context.Context, uint64, string) error
	DeliverSubscription(context.Context, uint64, domain.MessageRecipient, func() error) (bool, error)
}

func (s *Server) hasSubscriptionOutbox() bool {
	_, ok := s.store.(subscriptionOutboxStore)
	return ok
}

func (s *Server) deliverSubscription(ctx context.Context, store messageSubscriptionStore, jobID uint64, recipient domain.MessageRecipient, send func() error) (bool, error) {
	if jobID != 0 {
		return s.store.(subscriptionOutboxStore).DeliverSubscription(ctx, jobID, recipient, send)
	}
	if err := send(); err != nil {
		return false, err
	}
	return true, store.ConsumeMessageSubscriptionGrant(ctx, recipient.GrantID)
}

func (s *Server) RunSubscriptionOutbox(ctx context.Context) {
	store, ok := s.store.(subscriptionOutboxStore)
	if !ok || os.Getenv("APP_ENV") == "test" {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		s.processSubscriptionOutbox(ctx, store)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) processSubscriptionOutbox(ctx context.Context, store subscriptionOutboxStore) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	jobs, err := store.PendingSubscriptionJobs(queryCtx)
	cancel()
	if err != nil {
		log.Printf("read subscription outbox: %v", err)
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		jobCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err := s.sendSubscriptionJob(jobCtx, job)
		cancel()
		failure := ""
		if err != nil {
			failure = shortText(err.Error(), 500)
			if errors.Is(err, errSubscriptionBatch) {
				failure = "@next_batch"
			}
			if failure == "" {
				failure = "通知发送失败"
			}
			log.Printf("subscription job %d pending retry: %s", job.ID, failure)
		}
		finishCtx, finishCancel := context.WithTimeout(ctx, 5*time.Second)
		if finishErr := store.FinishSubscriptionJob(finishCtx, job.ID, failure); finishErr != nil {
			log.Printf("finish subscription job %d: %v", job.ID, finishErr)
		}
		finishCancel()
	}
}

func (s *Server) sendSubscriptionJob(ctx context.Context, job domain.SubscriptionJob) error {
	switch job.Kind {
	case "request":
		var item domain.RentalRequest
		if err := json.Unmarshal(job.Payload, &item); err != nil {
			return err
		}
		return s.notifyAdminsOfRequest(ctx, item, job.ID)
	case "property":
		store, ok := s.store.(interface {
			GetProperty(context.Context, string) (domain.Property, error)
		})
		if !ok {
			return errors.New("property lookup unavailable")
		}
		item, err := store.GetProperty(ctx, job.SubjectID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if item.Status != domain.PropertyAvailable || item.IsMaterial {
			return nil
		}
		return s.notifyAreaSubscribers(ctx, item, job.ID)
	}
	return errors.New("unknown subscription job kind")
}

func (s *Server) manageSubscriptionJobs(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(interface {
		ListSubscriptionJobSummaries(context.Context) ([]domain.SubscriptionJobSummary, error)
		RetrySubscriptionJob(context.Context, uint64) error
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "通知任务服务不可用")
		return
	}
	if r.Method == http.MethodGet {
		items, err := store.ListSubscriptionJobSummaries(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "通知任务读取失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": items})
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "通知任务编号无效")
		return
	}
	if err := store.RetrySubscriptionJob(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "该任务当前不可重试，请刷新")
			return
		}
		writeError(w, http.StatusInternalServerError, "通知任务重试失败")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "subscription", strconv.FormatUint(id, 10), "重试通知任务", "")
	writeJSON(w, http.StatusOK, map[string]any{"code": 0})
}
