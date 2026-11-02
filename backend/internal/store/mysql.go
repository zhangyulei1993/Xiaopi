package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"xiaopi/backend/internal/domain"
)

type MySQL struct{ db *sql.DB }

func OpenMySQL() (*MySQL, error) {
	host := env("MYSQL_HOST", "127.0.0.1")
	port := env("MYSQL_PORT", "3306")
	user := env("MYSQL_USER", "xiaopi_user")
	password := env("MYSQL_PASSWORD", "xiaopi_pass")
	database := env("MYSQL_DATABASE", "xiaopi_platform")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4", user, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &MySQL{db: db}, nil
}

func (s *MySQL) Close() error { return s.db.Close() }

func (s *MySQL) ListProperties(ctx context.Context) ([]domain.Property, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,community,location,layout,rent_monthly,fee_text,description,status,owner_id,created_at,updated_at FROM properties ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Property, 0)
	for rows.Next() {
		var p domain.Property
		if err := rows.Scan(&p.ID, &p.Title, &p.Community, &p.Location, &p.Layout, &p.RentMonthly, &p.FeeText, &p.Description, &p.Status, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		media, err := s.listPropertyMedia(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Media = media
	}
	return items, nil
}

func (s *MySQL) listPropertyMedia(ctx context.Context, propertyID string) ([]domain.PropertyMedia, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,property_id,media_type,url,sort_order FROM property_media WHERE property_id=? ORDER BY sort_order,id`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PropertyMedia, 0)
	for rows.Next() {
		var media domain.PropertyMedia
		if err := rows.Scan(&media.ID, &media.PropertyID, &media.Type, &media.URL, &media.Sort); err != nil {
			return nil, err
		}
		items = append(items, media)
	}
	return items, rows.Err()
}

func (s *MySQL) CreateProperty(ctx context.Context, p domain.Property) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO properties (id,title,community,location,layout,rent_monthly,fee_text,description,status,owner_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Title, p.Community, p.Location, p.Layout, p.RentMonthly, p.FeeText, p.Description, p.Status, p.OwnerID, p.CreatedAt, p.UpdatedAt)
	return err
}

func (s *MySQL) UpdateProperty(ctx context.Context, p domain.Property) error {
	_, err := s.db.ExecContext(ctx, `UPDATE properties SET title=?,community=?,location=?,layout=?,rent_monthly=?,fee_text=?,description=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, p.Title, p.Community, p.Location, p.Layout, p.RentMonthly, p.FeeText, p.Description, p.ID)
	return err
}

func (s *MySQL) ListPropertyLeads(ctx context.Context) ([]domain.PropertyLead, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_name,source_phone,community,location,layout,expected_rent_monthly,note,status,COALESCE(converted_property_id,''),created_at,updated_at FROM property_leads ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PropertyLead, 0)
	for rows.Next() {
		var lead domain.PropertyLead
		if err := rows.Scan(&lead.ID, &lead.SourceName, &lead.SourcePhone, &lead.Community, &lead.Location, &lead.Layout, &lead.ExpectedRentMonthly, &lead.Note, &lead.Status, &lead.ConvertedPropertyID, &lead.CreatedAt, &lead.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, lead)
	}
	return items, rows.Err()
}

func (s *MySQL) CreatePropertyLead(ctx context.Context, lead domain.PropertyLead) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO property_leads (id,source_name,source_phone,community,location,layout,expected_rent_monthly,note,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, lead.ID, lead.SourceName, lead.SourcePhone, lead.Community, lead.Location, lead.Layout, lead.ExpectedRentMonthly, lead.Note, lead.Status, lead.CreatedAt, lead.UpdatedAt)
	return err
}

func (s *MySQL) AddPropertyStatusChange(ctx context.Context, change domain.PropertyStatusChange) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO property_status_history (id,property_id,from_status,to_status,operator_id,note,created_at) VALUES (?,?,?,?,?,?,?)`, change.ID, change.PropertyID, change.From, change.To, change.OperatorID, change.Note, change.CreatedAt)
	return err
}

func (s *MySQL) AddPropertyMedia(ctx context.Context, media domain.PropertyMedia) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO property_media (id,property_id,media_type,url,sort_order) VALUES (?,?,?,?,?)`, media.ID, media.PropertyID, media.Type, media.URL, media.Sort)
	return err
}

func (s *MySQL) UpdatePropertyStatus(ctx context.Context, propertyID, nextStatus, operatorID, note string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM properties WHERE id=? FOR UPDATE`, propertyID).Scan(&current); err != nil {
		return err
	}
	if !domain.CanTransitionPropertyStatus(current, nextStatus) {
		return domain.ErrInvalidPropertyStatusTransition
	}
	if _, err := tx.ExecContext(ctx, `UPDATE properties SET status=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, nextStatus, propertyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO property_status_history (id,property_id,from_status,to_status,operator_id,note) VALUES (?,?,?,?,?,?)`, fmt.Sprintf("%d", time.Now().UnixNano()), propertyID, current, nextStatus, operatorID, note); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) ConvertLeadToProperty(ctx context.Context, leadID string, p domain.Property) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO properties (id,title,community,location,layout,rent_monthly,fee_text,description,status,owner_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Title, p.Community, p.Location, p.Layout, p.RentMonthly, p.FeeText, p.Description, p.Status, p.OwnerID, p.CreatedAt, p.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE property_leads SET status='converted',converted_property_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, p.ID, leadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) UpsertUser(ctx context.Context, user domain.User) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id,open_id,nickname,avatar_url,phone,status) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE nickname=VALUES(nickname),avatar_url=VALUES(avatar_url),updated_at=CURRENT_TIMESTAMP`, user.ID, user.OpenID, user.Nickname, user.AvatarURL, user.Phone, user.Status)
	return err
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
