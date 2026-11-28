package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"xiaopi/backend/internal/domain"
)

func (s *MySQL) CompareAndSwapAppConfig(ctx context.Context, key string, expected, next []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current []byte
	err = tx.QueryRowContext(ctx, "SELECT config_value FROM app_configs WHERE config_key=? FOR UPDATE", key).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !bytes.Equal(expected, current) {
		return domain.ErrConfigConflict
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO app_configs (config_key,config_value) VALUES (?,?) ON DUPLICATE KEY UPDATE config_value=VALUES(config_value),updated_at=CURRENT_TIMESTAMP", key, next); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) DeleteCommunityDataAtomic(ctx context.Context, catalogBefore, plansBefore, catalogAfter, plansAfter []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, change := range []struct {
		key           string
		before, after []byte
	}{
		{"doorplate_catalog", catalogBefore, catalogAfter},
		{"doorplate_collection_plans", plansBefore, plansAfter},
	} {
		var current []byte
		err := tx.QueryRowContext(ctx, "SELECT config_value FROM app_configs WHERE config_key=? FOR UPDATE", change.key).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !bytes.Equal(current, change.before) {
			return domain.ErrConfigConflict
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO app_configs(config_key,config_value) VALUES (?,?) ON DUPLICATE KEY UPDATE config_value=VALUES(config_value),updated_at=CURRENT_TIMESTAMP", change.key, change.after); err != nil {
			return err
		}
	}
	return tx.Commit()
}
