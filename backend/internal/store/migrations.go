package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyMigrations applies each SQL file once and records it in schema_migrations.
// Migrations intentionally live beside the backend source so a released compose
// deployment and a local run share the exact same schema definition.
func ApplyMigrations(db *sql.DB, dir string) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var lockName string
	if err := conn.QueryRowContext(ctx, "SELECT CONCAT('xiaopi:migrate:', LEFT(SHA2(DATABASE(),256),40))").Scan(&lockName); err != nil {
		return err
	}
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,30)", lockName).Scan(&locked); err != nil {
		return err
	}
	if locked != 1 {
		return fmt.Errorf("migration lock unavailable")
	}
	defer conn.ExecContext(ctx, "DO RELEASE_LOCK(?)", lockName)

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		var applied string
		err := conn.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		var recoverable bool
		switch name {
		case "028_atomic_property_edit.sql":
			recoverable = true
			for _, column := range []struct{ table, name, definition string }{
				{"properties", "edit_revision", "INT UNSIGNED NOT NULL DEFAULT 0"},
				{"properties", "last_edit_commit", "VARCHAR(80) NOT NULL DEFAULT ''"},
				{"property_media", "edit_batch", "VARCHAR(80) NOT NULL DEFAULT ''"},
			} {
				if err = ensureMigrationColumn(ctx, conn, column.table, column.name, column.definition); err != nil {
					break
				}
			}
		case "029_property_edit_commit_digest.sql":
			recoverable = true
			err = ensureMigrationColumn(ctx, conn, "properties", "last_edit_digest", "CHAR(64) NOT NULL DEFAULT ''")
		case "030_staged_media_retention.sql":
			recoverable = true
			err = recoverStagedMediaMigration(ctx, conn)
		case "032_temporary_session_version.sql":
			recoverable = true
			err = ensureMigrationColumn(ctx, conn, "temporary_access_codes", "session_version", "BIGINT UNSIGNED NOT NULL DEFAULT 1")
		case "034_subscription_delivery_results.sql":
			recoverable = true
			err = ensureMigrationColumn(ctx, conn, "subscription_deliveries", "result", "VARCHAR(16) NOT NULL DEFAULT 'uncertain'")
			if err == nil {
				err = ensureMigrationColumn(ctx, conn, "subscription_deliveries", "last_error", "VARCHAR(500) NOT NULL DEFAULT ''")
			}
		case "035_subscription_job_created.sql":
			recoverable = true
			err = ensureMigrationColumn(ctx, conn, "subscription_outbox", "created_at", "TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP")
			if err == nil {
				err = ensureMigrationColumn(ctx, conn, "subscription_outbox", "grant_ceiling", "BIGINT UNSIGNED NOT NULL DEFAULT 0")
			}
		case "033_subscription_job_results.sql":
			recoverable = true
			err = ensureMigrationColumn(ctx, conn, "subscription_outbox", "result", "VARCHAR(16) NOT NULL DEFAULT ''")
			if err == nil {
				err = ensureMigrationColumn(ctx, conn, "subscription_outbox", "last_error", "VARCHAR(500) NOT NULL DEFAULT ''")
			}
		}
		if name == "035_subscription_job_created.sql" && err == nil {
			_, err = conn.ExecContext(ctx, "UPDATE subscription_outbox SET result='failed',last_error='历史任务缺少授权范围，请核实后处理',finished_at=CURRENT_TIMESTAMP WHERE grant_ceiling=0 AND finished_at IS NULL")
		}
		if recoverable {
			if err != nil {
				return fmt.Errorf("apply migration %s: %w", name, err)
			}
			if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
				return err
			}
			continue
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err = tx.Exec(string(body)); err == nil {
			_, err = tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

func recoverStagedMediaMigration(ctx context.Context, conn *sql.Conn) error {
	for _, column := range []struct{ name, definition string }{
		{"staged_at", "TIMESTAMP NULL DEFAULT NULL"},
		{"staged_discarded", "BOOLEAN NOT NULL DEFAULT FALSE"},
	} {
		if err := ensureMigrationColumn(ctx, conn, "property_media", column.name, column.definition); err != nil {
			return err
		}
	}
	var indexed bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='property_media' AND index_name='idx_property_media_staged_at')").Scan(&indexed); err != nil {
		return err
	}
	if !indexed {
		if _, err := conn.ExecContext(ctx, "ALTER TABLE property_media ADD INDEX idx_property_media_staged_at (staged_at)"); err != nil {
			return err
		}
	}
	_, err := conn.ExecContext(ctx, "UPDATE property_media SET staged_at=CURRENT_TIMESTAMP WHERE edit_batch<>'' AND staged_at IS NULL")
	return err
}

func ensureMigrationColumn(ctx context.Context, conn *sql.Conn, table, name, definition string) error {
	var exists bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?)", table, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := conn.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+name+" "+definition)
	return err
}
