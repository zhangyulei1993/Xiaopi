package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"xiaopi/backend/internal/domain"
)

func enqueueSubscription(ctx context.Context, tx *sql.Tx, kind, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO subscription_outbox (kind,subject_id,payload,grant_ceiling) SELECT ?,?,?,COALESCE(MAX(id),0) FROM message_subscription_grants", kind, subject, data)
	return err
}

func (s *MySQL) PendingSubscriptionJobs(ctx context.Context) ([]domain.SubscriptionJob, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,kind,subject_id,payload FROM subscription_outbox WHERE finished_at IS NULL AND next_attempt<=NOW() ORDER BY next_attempt,id LIMIT 20")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []domain.SubscriptionJob
	for rows.Next() {
		var job domain.SubscriptionJob
		if err := rows.Scan(&job.ID, &job.Kind, &job.SubjectID, &job.Payload); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *MySQL) FinishSubscriptionJob(ctx context.Context, id uint64, failure string) error {
	if failure == "@next_batch" {
		_, err := s.db.ExecContext(ctx, "UPDATE subscription_outbox SET next_attempt=DATE_ADD(NOW(),INTERVAL 10 SECOND) WHERE id=? AND finished_at IS NULL", id)
		return err
	}
	if failure == "" {
		_, err := s.db.ExecContext(ctx, "UPDATE subscription_outbox SET result=IF(EXISTS(SELECT 1 FROM subscription_deliveries WHERE job_id=? AND result='uncertain'),'uncertain','completed'),last_error=IF(EXISTS(SELECT 1 FROM subscription_deliveries WHERE job_id=? AND result='uncertain'),'部分接收人发送结果待核实',''),finished_at=CURRENT_TIMESTAMP WHERE id=? AND finished_at IS NULL", id, id, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE subscription_outbox SET result=IF(attempts>=23,'failed','retrying'),finished_at=IF(attempts>=23,CURRENT_TIMESTAMP,NULL),attempts=attempts+1,last_error=?,next_attempt=DATE_ADD(NOW(),INTERVAL 5 MINUTE) WHERE id=? AND finished_at IS NULL", failure, id)
	return err
}

func (s *MySQL) ListSubscriptionJobSummaries(ctx context.Context) ([]domain.SubscriptionJobSummary, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,kind,subject_id,CASE WHEN result<>'' THEN result WHEN finished_at IS NULL THEN 'pending' ELSE 'unknown' END,attempts,last_error FROM subscription_outbox ORDER BY (result='failed') DESC,id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.SubscriptionJobSummary, 0)
	for rows.Next() {
		var item domain.SubscriptionJobSummary
		if err := rows.Scan(&item.ID, &item.Kind, &item.SubjectID, &item.Result, &item.Attempts, &item.LastError); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) RetrySubscriptionJob(ctx context.Context, id uint64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE subscription_outbox SET result='pending',finished_at=NULL,attempts=0,next_attempt=NOW(),grant_ceiling=IF(grant_ceiling=0,(SELECT COALESCE(MAX(id),0) FROM message_subscription_grants),grant_ceiling) WHERE id=? AND result='failed'", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *MySQL) DeliverSubscription(ctx context.Context, jobID uint64, recipient domain.MessageRecipient, send func() error) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Serialize attempts for the job and for the one-time grant.
	var job uint64
	err = tx.QueryRowContext(ctx, "SELECT id FROM subscription_outbox WHERE id=? AND finished_at IS NULL FOR UPDATE", jobID).Scan(&job)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var delivered bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM subscription_deliveries WHERE job_id=? AND user_id=?)", jobID, recipient.UserID).Scan(&delivered); err != nil {
		return false, err
	}
	if delivered {
		return false, nil
	}
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM message_subscription_grants WHERE id=? FOR UPDATE", recipient.GrantID).Scan(&status); err != nil {
		return false, err
	}
	if status != "active" {
		return false, nil
	}

	// Reserve the grant durably before contacting WeChat. A crash after this
	// commit leaves an uncertain delivery which must not be sent twice.
	if _, err := tx.ExecContext(ctx, "UPDATE message_subscription_grants SET status='uncertain' WHERE id=?", recipient.GrantID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO subscription_deliveries(job_id,user_id) VALUES (?,?)", jobID, recipient.UserID); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	if err := send(); err != nil {
		var notSent *domain.SubscriptionNotSent
		if errors.As(err, &notSent) {
			release, releaseErr := s.db.BeginTx(ctx, nil)
			if releaseErr != nil {
				return false, releaseErr
			}
			defer release.Rollback()
			grantStatus := "active"
			if notSent.InvalidGrant {
				grantStatus = "invalid"
			}
			if _, releaseErr = release.ExecContext(ctx, "UPDATE message_subscription_grants SET status=? WHERE id=? AND status='uncertain'", grantStatus, recipient.GrantID); releaseErr != nil {
				return false, releaseErr
			}
			if _, releaseErr = release.ExecContext(ctx, "DELETE FROM subscription_deliveries WHERE job_id=? AND user_id=? AND result='uncertain'", jobID, recipient.UserID); releaseErr != nil {
				return false, releaseErr
			}
			if releaseErr = release.Commit(); releaseErr != nil {
				return false, releaseErr
			}
			if notSent.InvalidGrant {
				return false, err
			}
			if notSent.Configuration {
				_, markErr := s.db.ExecContext(ctx, "UPDATE subscription_outbox SET result='failed',last_error=?,finished_at=CURRENT_TIMESTAMP WHERE id=?", err.Error(), jobID)
				if markErr != nil {
					return false, markErr
				}
			}
			return false, err
		}
		_, saveErr := s.db.ExecContext(ctx, "UPDATE subscription_deliveries SET last_error=? WHERE job_id=? AND user_id=?", err.Error(), jobID, recipient.UserID)
		if saveErr != nil {
			return false, saveErr
		}
		return false, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE message_subscription_grants SET status='used',used_at=CURRENT_TIMESTAMP WHERE id=?", recipient.GrantID)
	if err != nil {
		return false, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE subscription_deliveries SET result='sent',last_error='' WHERE job_id=? AND user_id=?", jobID, recipient.UserID)
	return err == nil, err

}
