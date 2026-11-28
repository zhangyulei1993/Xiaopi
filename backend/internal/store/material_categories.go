package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"xiaopi/backend/internal/domain"
)

func (s *MySQL) RenameMaterialCategoryAtomic(ctx context.Context, from, to string, oldCategories, oldLocations, categories, locations []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, change := range []struct {
		key           string
		before, after []byte
	}{
		{"doorplate_community_locations", oldLocations, locations},
		{"property_material_categories", oldCategories, categories},
	} {
		var current []byte
		err := tx.QueryRowContext(ctx, "SELECT config_value FROM app_configs WHERE config_key=? FOR UPDATE", change.key).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !bytes.Equal(current, change.before) {
			return domain.ErrPropertyEditConflict
		}
		if change.after != nil {
			if _, err := tx.ExecContext(ctx, "INSERT INTO app_configs (config_key,config_value) VALUES (?,?) ON DUPLICATE KEY UPDATE config_value=VALUES(config_value),updated_at=CURRENT_TIMESTAMP", change.key, change.after); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE properties SET
		title=IF(BINARY TRIM(title)=BINARY ?,?,title),
		location=IF(BINARY TRIM(location)=BINARY ?,?,location),
		community=IF(BINARY TRIM(community)=BINARY ?,?,community),
		material_category=?, edit_revision=edit_revision+1,updated_at=CURRENT_TIMESTAMP
		WHERE is_material=TRUE AND BINARY TRIM(material_category)=BINARY ?`, from, to, from, to, from, to, to, from); err != nil {
		return err
	}
	return tx.Commit()
}

func lockMaterialCategory(ctx context.Context, tx *sql.Tx, name string) error {
	var present bool
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(JSON_CONTAINS(config_value, JSON_QUOTE(?), '$.communities'),FALSE) FROM app_configs WHERE config_key='property_material_categories' FOR UPDATE", name).Scan(&present)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !present) {
		return domain.ErrPropertyEditConflict
	}
	return err
}

func (s *MySQL) ChangeMaterialCategoriesAtomic(ctx context.Context, before, after []byte, removed []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current []byte
	if err := tx.QueryRowContext(ctx, "SELECT config_value FROM app_configs WHERE config_key='property_material_categories' FOR UPDATE").Scan(&current); err != nil {
		return err
	}
	if !bytes.Equal(current, before) {
		return domain.ErrPropertyEditConflict
	}
	for _, name := range removed {
		var used bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM properties WHERE is_material=TRUE AND material_category=?)", name).Scan(&used); err != nil {
			return err
		}
		if used {
			return domain.ErrPropertyEditConflict
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE app_configs SET config_value=?,updated_at=CURRENT_TIMESTAMP WHERE config_key='property_material_categories'", after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) NormalizeMaterialCategoriesAtomic(ctx context.Context, before, after []byte, updates []domain.Property) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current []byte
	err = tx.QueryRowContext(ctx, "SELECT config_value FROM app_configs WHERE config_key='property_material_categories' FOR UPDATE").Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !bytes.Equal(current, before) {
		return domain.ErrPropertyEditConflict
	}
	for _, property := range updates {
		var revision int
		if err := tx.QueryRowContext(ctx, "SELECT edit_revision FROM properties WHERE id=? AND is_material=TRUE FOR UPDATE", property.ID).Scan(&revision); err != nil {
			return err
		}
		if revision != property.EditRevision {
			return domain.ErrPropertyEditConflict
		}
		if err := updatePropertyFields(ctx, tx, property); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO app_configs(config_key,config_value) VALUES ('property_material_categories',?) ON DUPLICATE KEY UPDATE config_value=VALUES(config_value),updated_at=CURRENT_TIMESTAMP", after); err != nil {
		return err
	}
	return tx.Commit()
}
