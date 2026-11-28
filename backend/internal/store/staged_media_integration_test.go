package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"xiaopi/backend/internal/domain"
)

func TestStagedMediaCleanupMySQL(t *testing.T) {
	dsn := os.Getenv("XIAOPI_STAGED_MEDIA_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL test DSN not configured")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.DBName = ""
	root, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx := context.Background()
	schema := fmt.Sprintf("xiaopi_staged_media_test_%d", time.Now().UnixNano())
	exec := func(db *sql.DB, query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(root, "CREATE DATABASE "+schema)
	defer root.ExecContext(ctx, "DROP DATABASE "+schema)
	cfg.DBName = schema
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec(db, "CREATE TABLE properties (id VARCHAR(36) PRIMARY KEY)")
	exec(db, "CREATE TABLE property_media (id VARCHAR(36) PRIMARY KEY, property_id VARCHAR(36), url TEXT, edit_batch VARCHAR(80))")
	migration, err := os.ReadFile("../../migrations/030_staged_media_retention.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after the DDL commits and before the migration ledger is written.
	exec(db, strings.SplitN(string(migration), ";", 2)[0])
	migrationDir := t.TempDir()
	for _, name := range []string{"030_staged_media_retention.sql", "031_subscription_outbox.sql", "033_subscription_job_results.sql", "034_subscription_delivery_results.sql"} {
		body, readErr := os.ReadFile("../../migrations/" + name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(filepath.Join(migrationDir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := ApplyMigrations(db, migrationDir); err != nil {
			t.Fatalf("migration retry %d: %v", attempt, err)
		}
	}
	exec(db, "INSERT INTO properties VALUES ('property'), ('material')")
	exec(db, `INSERT INTO property_media VALUES
		('expired','property','/uploads/expired.mp4','batch',NOW()-INTERVAL 3 DAY,FALSE),
		('shared','property','/uploads/shared.mp4','batch',NOW()-INTERVAL 3 DAY,FALSE),
		('retained','material','/uploads/shared.mp4','',NULL,FALSE),
		('recent','property','/uploads/recent.mp4','batch',NOW(),FALSE)`)
	store := &MySQL{db: db}
	cutoff := time.Now().Add(-48 * time.Hour)
	items, err := store.ExpiredStagedMedia(ctx, cutoff, 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("expected two expired staged files, got %v, %v", items, err)
	}
	expired := domain.PropertyMedia{ID: "expired", PropertyID: "property"}
	storageFailure := errors.New("simulated storage failure")
	if err := store.RemoveExpiredStagedMedia(ctx, expired, cutoff, func(context.Context, string) error { return storageFailure }); !errors.Is(err, storageFailure) {
		t.Fatalf("want retryable storage failure, got %v", err)
	}
	var discarded bool
	if err := db.QueryRowContext(ctx, "SELECT staged_discarded FROM property_media WHERE id='expired'").Scan(&discarded); err != nil || !discarded {
		t.Fatalf("discard must persist even on storage failure: %v %v", discarded, err)
	}
	var removed []string
	for _, item := range items {
		if err := store.RemoveExpiredStagedMedia(ctx, item, cutoff, func(_ context.Context, url string) error {
			removed = append(removed, url)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(removed) != 1 || removed[0] != "/uploads/expired.mp4" {
		t.Fatalf("unexpected physical deletion: %v", removed)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM property_media").Scan(&remaining); err != nil || remaining != 2 {
		t.Fatalf("expected retained and recent rows: %d %v", remaining, err)
	}
	t.Run("subscription delivery retry and single-use grant", func(t *testing.T) {
		exec(db, "CREATE TABLE message_subscription_grants (id BIGINT PRIMARY KEY, status VARCHAR(16), used_at TIMESTAMP NULL)")
		exec(db, "INSERT INTO message_subscription_grants (id,status) VALUES (1,'active'),(2,'active')")
		exec(db, "INSERT INTO subscription_outbox (id,kind,subject_id,payload) VALUES (1,'request','r1','{}'),(2,'request','r2','{}')")
		rcpt := domain.MessageRecipient{GrantID: 1, UserID: "admin"}
		if _, err := store.DeliverSubscription(ctx, 1, rcpt, func() error { return storageFailure }); !errors.Is(err, storageFailure) {
			t.Fatalf("failed delivery should retry: %v", err)
		}
		sends := 0
		send := func() error { sends++; return nil }
		if sent, err := store.DeliverSubscription(ctx, 1, rcpt, send); err != nil || sent {
			t.Fatalf("retry: %v %v", sent, err)
		}
		rcpt.GrantID = 2
		if sent, err := store.DeliverSubscription(ctx, 1, rcpt, send); err != nil || sent {
			t.Fatalf("duplicate job recipient: %v %v", sent, err)
		}
		rcpt.GrantID = 1
		if sent, err := store.DeliverSubscription(ctx, 2, rcpt, send); err != nil || sent {
			t.Fatalf("duplicate grant: %v %v", sent, err)
		}
		if sends != 0 {
			t.Fatalf("sent %d times", sends)
		}
	})
	t.Run("rename updates all three records together", func(t *testing.T) {
		exec(db, "CREATE TABLE app_configs (config_key VARCHAR(64) PRIMARY KEY,config_value JSON,updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
		exec(db, `ALTER TABLE properties ADD title VARCHAR(80), ADD location VARCHAR(80), ADD community VARCHAR(80), ADD material_category VARCHAR(80), ADD is_material BOOLEAN, ADD edit_revision INT DEFAULT 0, ADD updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
		exec(db, `UPDATE properties SET title='old',location='old',community='old',material_category='old',is_material=TRUE WHERE id='material'`)
		exec(db, `INSERT INTO app_configs(config_key,config_value) VALUES ('property_material_categories','{}'),('doorplate_community_locations','{}')`)
		if err := store.RenameMaterialCategoryAtomic(ctx, "old", "new", []byte("{}"), []byte("{}"), []byte("[1]"), []byte("invalid JSON")); err == nil {
			t.Fatal("expected invalid config failure")
		}
		var name string
		if err := db.QueryRowContext(ctx, "SELECT material_category FROM properties WHERE id='material'").Scan(&name); err != nil || name != "old" {
			t.Fatalf("partial rename: %q %v", name, err)
		}
		if err := store.RenameMaterialCategoryAtomic(ctx, "old", "new", []byte("{}"), []byte("{}"), []byte("[1]"), []byte("[2]")); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "SELECT material_category FROM properties WHERE id='material'").Scan(&name); err != nil || name != "new" {
			t.Fatalf("rename: %q %v", name, err)
		}
	})

	t.Run("fresh database startup migrations", func(t *testing.T) {
		fresh := schema + "_fresh"
		exec(root, "CREATE DATABASE "+fresh)
		defer root.ExecContext(ctx, "DROP DATABASE "+fresh)
		cfg.DBName = fresh
		freshDB, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		defer freshDB.Close()
		for attempt := 0; attempt < 2; attempt++ {
			if err := ApplyMigrations(freshDB, "../../migrations"); err != nil {
				t.Fatalf("startup %d: %v", attempt, err)
			}
		}

		t.Run("028 and 029 recover after committed DDL without ledger", func(t *testing.T) {
			exec(freshDB, "DELETE FROM schema_migrations WHERE version IN ('028_atomic_property_edit.sql','029_property_edit_commit_digest.sql')")
			exec(freshDB, "ALTER TABLE property_media DROP COLUMN edit_batch")
			exec(freshDB, "ALTER TABLE properties DROP COLUMN last_edit_digest")
			if err := ApplyMigrations(freshDB, "../../migrations"); err != nil {
				t.Fatal(err)
			}
		})
		freshStore := &MySQL{db: freshDB}
		t.Run("creation receipt, conflict and durable community work", func(t *testing.T) {
			p := domain.Property{ID: "task-first", Title: "home", Community: "小区甲", Status: domain.PropertyDraft, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			got, err := freshStore.CreatePropertyOnce(ctx, p, "task-one", "digest-one")
			if err != nil {
				t.Fatal(err)
			}
			p.ID = "task-retry"
			again, err := freshStore.CreatePropertyOnce(ctx, p, "task-one", "digest-one")
			if err != nil || again.ID != got.ID {
				t.Fatalf("retry duplicated: %v %v", again, err)
			}
			if _, err := freshStore.CreatePropertyOnce(ctx, p, "task-one", "changed-body"); !errors.Is(err, domain.ErrPropertyEditConflict) {
				t.Fatalf("changed task body accepted: %v", err)
			}
			var count int
			if err := freshDB.QueryRow("SELECT COUNT(*) FROM properties WHERE id IN ('task-first','task-retry')").Scan(&count); err != nil || count != 1 {
				t.Fatalf("created %d: %v", count, err)
			}
			restarted := &MySQL{db: freshDB}
			jobs, err := restarted.PendingCommunitySync(ctx)
			if err != nil || len(jobs) != 1 || jobs[0].Property.Community != "小区甲" {
				t.Fatalf("lost sync after restart: %v %v", jobs, err)
			}
			if err := restarted.FinishCommunitySync(ctx, jobs[0].ID, "temporary failure"); err != nil {
				t.Fatal(err)
			}
			jobsNow, err := restarted.PendingCommunitySync(ctx)
			if err != nil || len(jobsNow) != 0 {
				t.Fatalf("retry not delayed: %v %v", jobsNow, err)
			}
			if err := restarted.FinishCommunitySync(ctx, jobs[0].ID, ""); err != nil {
				t.Fatal(err)
			}
			exec(freshDB, "RENAME TABLE community_sync_jobs TO community_sync_unavailable")
			p.ID = "must-rollback"
			failure := freshStore.CreateProperty(ctx, p)
			exec(freshDB, "RENAME TABLE community_sync_unavailable TO community_sync_jobs")
			if failure == nil {
				t.Fatal("accepted listing without durable sync")
			}
			if err := freshDB.QueryRow("SELECT COUNT(*) FROM properties WHERE id='must-rollback'").Scan(&count); err != nil || count != 0 {
				t.Fatalf("partial listing commit: %d %v", count, err)
			}
			exec(freshDB, "DELETE FROM properties WHERE id='task-first'")
		})
		expiration := time.Now().Add(time.Hour).Truncate(time.Second)
		exec(freshDB, "INSERT INTO users (id,open_id,nickname,avatar_url,phone,status) VALUES ('wx','openid','','','13800138000','active')")
		if _, err := freshStore.UpsertTemporaryAccess(ctx, "13800138000", "hash", "123456", "admin", expiration); err != nil {
			t.Fatal(err)
		}
		user, grant, err := freshStore.LoginTemporaryUser(ctx, "13800138000", "hash", time.Now())
		if err != nil || !grant.ExpiresAt.Equal(expiration) {
			t.Fatalf("grant expiration: %#v %v", grant, err)
		}
		if err := freshStore.ValidateTemporarySession(ctx, user.ID, grant.ID, grant.Version); err != nil {
			t.Fatal(err)
		}
		if err := freshStore.SetTemporaryAccessStatus(ctx, "13800138000", "revoked"); err != nil {
			t.Fatal(err)
		}
		if err := freshStore.ValidateTemporarySession(ctx, user.ID, grant.ID, grant.Version); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("revoked grant validated: %v", err)
		}

		now := time.Now()
		for _, property := range []domain.Property{
			{ID: "offline", Title: "offline", Community: "test", Status: domain.PropertyOffline, CreatedAt: now, UpdatedAt: now},
			{ID: "material", Title: "material", Community: "test", Status: domain.PropertyDraft, IsMaterial: true, CreatedAt: now, UpdatedAt: now},
		} {
			if err := insertProperty(ctx, freshDB, property); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := freshStore.CreateAppointmentOnce(ctx, domain.RentalRequest{PropertyID: "offline"}); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("offline appointment accepted: %v", err)
		}
		for _, ref := range []struct{ id, property string }{{"first", "offline"}, {"second", "material"}} {
			exec(freshDB, "INSERT INTO property_media (id,property_id,media_type,url) VALUES (?,?,'image','/uploads/shared.jpg')", ref.id, ref.property)
		}
		if err := freshStore.DeleteProperty(ctx, "offline"); err != nil {
			t.Fatal(err)
		}
		cutoff := time.Now()
		deletes := 0
		remove := func(context.Context, string) error { deletes++; return nil }
		if err := freshStore.RemoveExpiredStagedMedia(ctx, domain.PropertyMedia{ID: "first", PropertyID: "offline"}, cutoff, remove); err != nil || deletes != 0 {
			t.Fatalf("shared storage must survive the first delete: %d %v", deletes, err)
		}
		if err := freshStore.DeleteProperty(ctx, "material"); err != nil {
			t.Fatal(err)
		}
		if err := freshStore.RemoveExpiredStagedMedia(ctx, domain.PropertyMedia{ID: "second", PropertyID: "material"}, time.Now(), remove); err != nil || deletes != 1 {
			t.Fatalf("unreferenced storage should be removed once: %d %v", deletes, err)
		}
		exec(freshDB, "INSERT INTO subscription_outbox (id,kind,subject_id,payload,attempts) VALUES (1,'request','test','{}',23)")
		if err := freshStore.FinishSubscriptionJob(ctx, 1, "simulated failure"); err != nil {
			t.Fatal(err)
		}
		jobs, err := freshStore.ListSubscriptionJobSummaries(ctx)
		if err != nil || len(jobs) != 1 || jobs[0].Result != "failed" || jobs[0].LastError != "simulated failure" {
			t.Fatalf("missing failure status: %#v %v", jobs, err)
		}
		if err := freshStore.RetrySubscriptionJob(ctx, 1); err != nil {
			t.Fatal(err)
		}
		pending, err := freshStore.PendingSubscriptionJobs(ctx)
		if err != nil || len(pending) != 1 {
			t.Fatalf("job not queued: %v %v", pending, err)
		}

	})

}
