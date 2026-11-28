package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"xiaopi/backend/internal/domain"
)

// The receipt and property commit together. A retry cannot create another draft
// or silently use a different request body under the same task identifier.
func (s *MySQL) CreatePropertyOnce(ctx context.Context, p domain.Property, task, digest string) (domain.Property, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	snapshot, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO property_creation_tasks(task_id,digest,property_id,snapshot) VALUES (?,?,?,?)", task, digest, p.ID, snapshot)
	if err != nil {
		return p, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return p, err
	}
	if count == 0 {
		var savedDigest, id string
		var data []byte
		if err := tx.QueryRowContext(ctx, "SELECT digest,property_id,snapshot FROM property_creation_tasks WHERE task_id=? FOR UPDATE", task).Scan(&savedDigest, &id, &data); err != nil {
			return p, err
		}
		if savedDigest != digest {
			return p, domain.ErrPropertyEditConflict
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM properties WHERE id=?)", id).Scan(&exists); err != nil {
			return p, err
		}
		if !exists {
			return p, domain.ErrPropertyEditConflict
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return p, err
		}
		return p, tx.Commit()
	}
	if p.IsMaterial {
		if err := lockMaterialCategory(ctx, tx, p.MaterialCategory); err != nil {
			return p, err
		}
	}
	if err := insertProperty(ctx, tx, p); err != nil {
		return p, err
	}
	if err := enqueueCommunitySync(ctx, tx, p); err != nil {
		return p, err
	}
	return p, tx.Commit()
}

func enqueueCommunitySync(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, p domain.Property) error {
	if p.IsMaterial || p.Community == "" {
		return nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "INSERT INTO community_sync_jobs(payload) VALUES (?)", data)
	return err
}

type CommunitySyncJob struct {
	ID       uint64
	Property domain.Property
}

func (s *MySQL) PendingCommunitySync(ctx context.Context) ([]CommunitySyncJob, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,payload FROM community_sync_jobs WHERE next_attempt<=NOW() ORDER BY id LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []CommunitySyncJob
	for rows.Next() {
		var job CommunitySyncJob
		var data []byte
		if err := rows.Scan(&job.ID, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &job.Property); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *MySQL) FinishCommunitySync(ctx context.Context, id uint64, failure string) error {
	if failure == "" {
		_, err := s.db.ExecContext(ctx, "DELETE FROM community_sync_jobs WHERE id=?", id)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE community_sync_jobs SET last_error=?,next_attempt=DATE_ADD(NOW(),INTERVAL 1 MINUTE) WHERE id=?", failure, id)
	return err
}
