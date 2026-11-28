package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"xiaopi/backend/internal/domain"
)

func (s *MySQL) ExpiredStagedMedia(ctx context.Context, cutoff time.Time, limit int) ([]domain.PropertyMedia, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,property_id FROM property_media WHERE staged_at<=FROM_UNIXTIME(?) AND edit_batch<>'' ORDER BY staged_at,id LIMIT ?", cutoff.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.PropertyMedia
	for rows.Next() {
		var item domain.PropertyMedia
		if err := rows.Scan(&item.ID, &item.PropertyID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) RemoveExpiredStagedMedia(ctx context.Context, item domain.PropertyMedia, cutoff time.Time, remove func(context.Context, string) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Saving and conversion lock the property before changing media.
	var id string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM properties WHERE id=? FOR UPDATE", item.PropertyID).Scan(&id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var objectURL string
	err = tx.QueryRowContext(ctx, "SELECT url FROM property_media WHERE id=? AND property_id=? AND staged_at<=FROM_UNIXTIME(?) AND edit_batch<>'' FOR UPDATE", item.ID, item.PropertyID, cutoff.Unix()).Scan(&objectURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var referenced bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM property_media WHERE url=? AND id<>?)", objectURL, item.ID).Scan(&referenced); err != nil {
		return err
	}
	if referenced {
		if _, err := tx.ExecContext(ctx, "DELETE FROM property_media WHERE id=?", item.ID); err != nil {
			return err
		}
		return tx.Commit()
	}
	// Commit the discard before deleting the physical file. A failed storage
	// deletion can then retry without making the upload available to Save.
	if _, err := tx.ExecContext(ctx, "UPDATE property_media SET staged_discarded=TRUE WHERE id=?", item.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := remove(ctx, objectURL); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM property_media WHERE id=? AND staged_discarded=TRUE AND url=?", item.ID, objectURL)
	return err
}
