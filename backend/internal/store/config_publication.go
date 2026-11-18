package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"time"
	"xiaopi/backend/internal/domain"
)

// Publish configuration and audit snapshot atomically. A database row lock also
// protects against publishes from a different API process.
func (s *MySQL) PublishPublicConfig(ctx context.Context, expected string, config domain.PublicConfig, operator, restoredFrom string) (publication domain.ConfigPublication, resultErr error) {
	defer func() {
		var databaseError *mysql.MySQLError
		if errors.As(resultErr, &databaseError) && (databaseError.Number == 1213 || databaseError.Number == 1205) {
			resultErr = domain.ErrConfigConflict
		}
	}()
	var empty domain.ConfigPublication
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	initialConfig := domain.DefaultPublicConfig()
	initialConfig.ContactPhone = os.Getenv("CONTACT_PHONE")
	initialConfig.Normalize()
	initial, _ := json.Marshal(initialConfig)
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO app_configs (config_key,config_value) VALUES ('miniapp_public',?)`, initial); err != nil {
		return empty, err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT config_value FROM app_configs WHERE config_key='miniapp_public' FOR UPDATE`).Scan(&raw); err != nil {
		return empty, err
	}
	before := domain.DefaultPublicConfig()
	if err = json.Unmarshal(raw, &before); err != nil {
		return empty, err
	}
	before.Normalize()
	if domain.ConfigRevision(before) != expected {
		return empty, domain.ErrConfigConflict
	}
	now := time.Now().UTC()
	entry := domain.ConfigPublication{ID: fmt.Sprintf("%020d", now.UnixNano()), Operator: operator, CreatedAt: now, Revision: domain.ConfigRevision(config), RestoredFrom: restoredFrom, Before: before, After: config}
	snapshot, err := json.Marshal(entry)
	if err != nil {
		return empty, err
	}
	value, err := json.Marshal(config)
	if err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE app_configs SET config_value=?,updated_at=CURRENT_TIMESTAMP WHERE config_key='miniapp_public'`, value); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO app_configs (config_key,config_value) VALUES (?,?)`, "miniapp_publish_"+entry.ID, snapshot); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return entry, nil
}

func (s *MySQL) ListConfigPublications(ctx context.Context, before string) ([]domain.ConfigPublication, error) {
	cursor := "miniapp_publish_~"
	if before != "" {
		cursor = "miniapp_publish_" + before
	}
	rows, err := s.db.QueryContext(ctx, `SELECT config_value FROM app_configs WHERE LEFT(config_key,CHAR_LENGTH('miniapp_publish_'))='miniapp_publish_' AND BINARY config_key < BINARY ? ORDER BY BINARY config_key DESC LIMIT 50`, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]domain.ConfigPublication, 0)
	for rows.Next() {
		var raw []byte
		var entry domain.ConfigPublication
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Failed attempts are audit-only: they never update the effective configuration.
func (s *MySQL) RecordConfigFailure(ctx context.Context, entry domain.ConfigPublication) error {
	entry.ID = fmt.Sprintf("%020d", time.Now().UTC().UnixNano())
	entry.CreatedAt = time.Now().UTC()
	entry.Status = "failed"
	value, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.UpsertAppConfig(ctx, "miniapp_publish_"+entry.ID, value)
}
