package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
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
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&multiStatements=true", user, password, host, port, database)
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

// DB exposes the connection solely for startup migration orchestration.
func (s *MySQL) DB() *sql.DB { return s.db }

func (s *MySQL) ListProperties(ctx context.Context) ([]domain.Property, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,community,location,area_label,building_no,unit_no,room_no,latitude,longitude,layout,rent_monthly,fee_text,rent_type,listing_source,remaining_lease_months,transfer_deposit,available_date,deposit_payment,utilities_text,area_sqm,floor_info,orientation,elevator,metro_minutes,verification_date,rules_text,description,status,owner_id,created_at,updated_at FROM properties ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Property, 0)
	for rows.Next() {
		var p domain.Property
		if err := rows.Scan(&p.ID, &p.Title, &p.Community, &p.Location, &p.AreaLabel, &p.BuildingNo, &p.UnitNo, &p.RoomNo, &p.Latitude, &p.Longitude, &p.Layout, &p.RentMonthly, &p.FeeText, &p.RentType, &p.ListingSource, &p.RemainingLeaseMonths, &p.TransferDeposit, &p.AvailableDate, &p.DepositPayment, &p.UtilitiesText, &p.AreaSqm, &p.FloorInfo, &p.Orientation, &p.Elevator, &p.MetroMinutes, &p.VerificationDate, &p.RulesText, &p.Description, &p.Status, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO properties (id,title,community,location,area_label,building_no,unit_no,room_no,latitude,longitude,layout,rent_monthly,fee_text,rent_type,listing_source,remaining_lease_months,transfer_deposit,available_date,deposit_payment,utilities_text,area_sqm,floor_info,orientation,elevator,metro_minutes,verification_date,rules_text,description,status,owner_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Title, p.Community, p.Location, p.AreaLabel, p.BuildingNo, p.UnitNo, p.RoomNo, p.Latitude, p.Longitude, p.Layout, p.RentMonthly, p.FeeText, p.RentType, p.ListingSource, p.RemainingLeaseMonths, p.TransferDeposit, p.AvailableDate, p.DepositPayment, p.UtilitiesText, p.AreaSqm, p.FloorInfo, p.Orientation, p.Elevator, p.MetroMinutes, p.VerificationDate, p.RulesText, p.Description, p.Status, p.OwnerID, p.CreatedAt, p.UpdatedAt)
	return err
}

func (s *MySQL) UpdateProperty(ctx context.Context, p domain.Property) error {
	_, err := s.db.ExecContext(ctx, `UPDATE properties SET title=?,community=?,location=?,area_label=?,building_no=?,unit_no=?,room_no=?,latitude=?,longitude=?,layout=?,rent_monthly=?,fee_text=?,rent_type=?,listing_source=?,remaining_lease_months=?,transfer_deposit=?,available_date=?,deposit_payment=?,utilities_text=?,area_sqm=?,floor_info=?,orientation=?,elevator=?,metro_minutes=?,verification_date=?,rules_text=?,description=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, p.Title, p.Community, p.Location, p.AreaLabel, p.BuildingNo, p.UnitNo, p.RoomNo, p.Latitude, p.Longitude, p.Layout, p.RentMonthly, p.FeeText, p.RentType, p.ListingSource, p.RemainingLeaseMonths, p.TransferDeposit, p.AvailableDate, p.DepositPayment, p.UtilitiesText, p.AreaSqm, p.FloorInfo, p.Orientation, p.Elevator, p.MetroMinutes, p.VerificationDate, p.RulesText, p.Description, p.ID)
	return err
}

func (s *MySQL) ListBuildingRules(ctx context.Context, community string) ([]domain.BuildingRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,community,building_no,unit_count,floor_start,floor_count,households_per_floor FROM building_rules WHERE community=? ORDER BY CAST(building_no AS UNSIGNED), building_no`, community)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.BuildingRule, 0)
	for rows.Next() {
		var item domain.BuildingRule
		if err := rows.Scan(&item.ID, &item.Community, &item.BuildingNo, &item.UnitCount, &item.FloorStart, &item.FloorCount, &item.HouseholdsPerFloor); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) UpsertBuildingRule(ctx context.Context, rule domain.BuildingRule) error {
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("building-%d", time.Now().UnixNano())
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO building_rules (id,community,building_no,unit_count,floor_start,floor_count,households_per_floor) VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE unit_count=VALUES(unit_count),floor_start=VALUES(floor_start),floor_count=VALUES(floor_count),households_per_floor=VALUES(households_per_floor),updated_at=CURRENT_TIMESTAMP`, rule.ID, rule.Community, rule.BuildingNo, rule.UnitCount, rule.FloorStart, rule.FloorCount, rule.HouseholdsPerFloor)
	return err
}

// DeleteProperty removes a listing and its listing-specific traces in one
// transaction. Appointment records are intentionally retained: their
// property_id is cleared so an operator can still follow up with the customer
// after a listing has been removed.
func (s *MySQL) DeleteProperty(ctx context.Context, propertyID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM property_media WHERE property_id=?`, propertyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM property_favorites WHERE property_id=?`, propertyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_footprints WHERE property_id=?`, propertyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM property_status_history WHERE property_id=?`, propertyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rental_requests SET property_id='' WHERE property_id=?`, propertyID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM properties WHERE id=?`, propertyID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *MySQL) ListPropertyLeads(ctx context.Context) ([]domain.PropertyLead, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_name,source_phone,community,location,layout,expected_rent_monthly,note,follow_up_note,status,COALESCE(converted_property_id,''),created_at,updated_at FROM property_leads ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PropertyLead, 0)
	for rows.Next() {
		var lead domain.PropertyLead
		if err := rows.Scan(&lead.ID, &lead.SourceName, &lead.SourcePhone, &lead.Community, &lead.Location, &lead.Layout, &lead.ExpectedRentMonthly, &lead.Note, &lead.FollowUpNote, &lead.Status, &lead.ConvertedPropertyID, &lead.CreatedAt, &lead.UpdatedAt); err != nil {
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

func (s *MySQL) ListRentalRequests(ctx context.Context) ([]domain.RentalRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,request_type,property_id,phone,location,layout,budget_min,budget_max,move_in_date,note,follow_up_note,status,created_at,updated_at FROM rental_requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RentalRequest, 0)
	for rows.Next() {
		var item domain.RentalRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestType, &item.PropertyID, &item.Phone, &item.Location, &item.Layout, &item.BudgetMin, &item.BudgetMax, &item.MoveInDate, &item.Note, &item.FollowUpNote, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) UpdatePropertyLeadFollowUp(ctx context.Context, id, status, note string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE property_leads SET status=?,follow_up_note=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, note, id)
	return err
}

func (s *MySQL) UpdateRentalRequestFollowUp(ctx context.Context, id, status, note string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rental_requests SET status=?,follow_up_note=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, note, id)
	return err
}

func (s *MySQL) CreateOperationLog(ctx context.Context, item domain.OperationLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO operation_logs (id,operator_id,target_type,target_id,action,detail,created_at) VALUES (?,?,?,?,?,?,?)`, item.ID, item.OperatorID, item.TargetType, item.TargetID, item.Action, item.Detail, item.CreatedAt)
	return err
}

func (s *MySQL) ListOperationLogs(ctx context.Context, limit int) ([]domain.OperationLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,operator_id,target_type,target_id,action,detail,created_at FROM operation_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.OperationLog, 0)
	for rows.Next() {
		var item domain.OperationLog
		if err := rows.Scan(&item.ID, &item.OperatorID, &item.TargetType, &item.TargetID, &item.Action, &item.Detail, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) CreateRentalRequest(ctx context.Context, item domain.RentalRequest) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO rental_requests (id,user_id,request_type,property_id,phone,location,layout,budget_min,budget_max,move_in_date,note,status,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.UserID, item.RequestType, item.PropertyID, item.Phone, item.Location, item.Layout, item.BudgetMin, item.BudgetMax, item.MoveInDate, item.Note, item.Status, item.CreatedAt)
	return err
}

func (s *MySQL) ListFavoritePropertyIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT property_id FROM property_favorites WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (s *MySQL) SetPropertyFavorite(ctx context.Context, userID, propertyID string, favorite bool) error {
	if favorite {
		_, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO property_favorites (user_id,property_id) VALUES (?,?)`, userID, propertyID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM property_favorites WHERE user_id=? AND property_id=?`, userID, propertyID)
	return err
}

func (s *MySQL) ListFavoriteContacts(ctx context.Context) ([]domain.FavoriteContact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.user_id,u.nickname,u.avatar_url,u.phone,f.property_id,COALESCE(NULLIF(p.community,''),p.title),f.created_at FROM property_favorites f JOIN users u ON u.id=f.user_id JOIN properties p ON p.id=f.property_id ORDER BY f.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.FavoriteContact, 0)
	for rows.Next() {
		var item domain.FavoriteContact
		if err := rows.Scan(&item.UserID, &item.Nickname, &item.AvatarURL, &item.Phone, &item.PropertyID, &item.PropertyName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) RecordPropertyFootprint(ctx context.Context, userID, propertyID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_footprints (user_id,property_id) VALUES (?,?) ON DUPLICATE KEY UPDATE viewed_at=CURRENT_TIMESTAMP,view_count=view_count+1`, userID, propertyID)
	return err
}

func (s *MySQL) ListPropertyFootprints(ctx context.Context, userID string) ([]domain.Footprint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT property_id,viewed_at,view_count FROM user_footprints WHERE user_id=? ORDER BY viewed_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Footprint, 0)
	for rows.Next() {
		var item domain.Footprint
		if err := rows.Scan(&item.PropertyID, &item.ViewedAt, &item.ViewCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) ClearPropertyFootprints(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_footprints WHERE user_id=?`, userID)
	return err
}

func (s *MySQL) AddMessageSubscriptionGrant(ctx context.Context, userID, templateID, area string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO message_subscription_grants (user_id,template_id,area) VALUES (?,?,?)`, userID, templateID, area)
	return err
}
func (s *MySQL) ListAdminMessageRecipients(ctx context.Context, templateID string) ([]domain.MessageRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id,g.user_id,u.open_id,g.area FROM message_subscription_grants g JOIN users u ON u.id=g.user_id JOIN admin_users a ON a.user_id=g.user_id AND a.status='active' WHERE g.template_id=? AND g.status='active' AND g.id=(SELECT MIN(g2.id) FROM message_subscription_grants g2 WHERE g2.user_id=g.user_id AND g2.template_id=g.template_id AND g2.status='active') ORDER BY g.created_at LIMIT 50`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.MessageRecipient{}
	for rows.Next() {
		var item domain.MessageRecipient
		if err := rows.Scan(&item.GrantID, &item.UserID, &item.OpenID, &item.Area); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *MySQL) ListAreaMessageRecipients(ctx context.Context, templateID string, property domain.Property) ([]domain.MessageRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id,g.user_id,u.open_id,g.area FROM message_subscription_grants g JOIN users u ON u.id=g.user_id WHERE g.template_id=? AND g.status='active' AND g.area<>'' AND g.id=(SELECT MIN(g2.id) FROM message_subscription_grants g2 WHERE g2.user_id=g.user_id AND g2.template_id=g.template_id AND g2.area=g.area AND g2.status='active') AND (INSTR(?,g.area)>0 OR INSTR(?,g.area)>0 OR INSTR(?,g.area)>0) ORDER BY g.created_at LIMIT 200`, templateID, property.Location, property.Community, property.Title)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.MessageRecipient{}
	for rows.Next() {
		var item domain.MessageRecipient
		if err := rows.Scan(&item.GrantID, &item.UserID, &item.OpenID, &item.Area); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *MySQL) ConsumeMessageSubscriptionGrant(ctx context.Context, id uint64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE message_subscription_grants SET status='used',used_at=CURRENT_TIMESTAMP WHERE id=? AND status='active'`, id)
	return err
}
func (s *MySQL) ListUnnotifiedRentalRequests(ctx context.Context, userID string) ([]domain.RentalRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,request_type,property_id,phone,location,layout,budget_min,budget_max,move_in_date,note,follow_up_note,status,created_at,updated_at FROM rental_requests WHERE user_id=? AND admin_notified_at IS NULL AND status='new' ORDER BY created_at DESC LIMIT 20`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.RentalRequest{}
	for rows.Next() {
		var item domain.RentalRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestType, &item.PropertyID, &item.Phone, &item.Location, &item.Layout, &item.BudgetMin, &item.BudgetMax, &item.MoveInDate, &item.Note, &item.FollowUpNote, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *MySQL) MarkRentalRequestAdminNotified(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rental_requests SET admin_notified_at=CURRENT_TIMESTAMP WHERE id=?`, id)
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

// ReplacePropertyMedia persists one complete editor ordering. IDs omitted from
// the order are removed; IDs owned by another property are rejected.
func (s *MySQL) ReplacePropertyMedia(ctx context.Context, propertyID string, mediaIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM property_media WHERE property_id=? FOR UPDATE`, propertyID)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		existing[id] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(mediaIDs))
	for _, id := range mediaIDs {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("media does not belong to property")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("duplicate media id")
		}
		seen[id] = struct{}{}
	}
	for id := range existing {
		if _, keep := seen[id]; !keep {
			if _, err := tx.ExecContext(ctx, `DELETE FROM property_media WHERE id=? AND property_id=?`, id, propertyID); err != nil {
				return err
			}
		}
	}
	for sort, id := range mediaIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE property_media SET sort_order=? WHERE id=? AND property_id=?`, sort, id, propertyID); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	// The edit page can be reopened after a network retry. Repeating the target
	// status is intentionally idempotent rather than an operator-facing 409.
	if current == nextStatus {
		return tx.Commit()
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

func (s *MySQL) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, user)
	}
	return items, rows.Err()
}

func (s *MySQL) GetUser(ctx context.Context, userID string) (domain.User, error) {
	var user domain.User
	err := s.db.QueryRowContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at FROM users WHERE id=?`, userID).Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s *MySQL) UpdateUserProfile(ctx context.Context, userID, nickname, avatarURL string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET nickname=IF(?='',nickname,?),avatar_url=IF(?='',avatar_url,?),updated_at=CURRENT_TIMESTAMP WHERE id=?`, nickname, nickname, avatarURL, avatarURL, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *MySQL) BindUserPhone(ctx context.Context, userID, phone string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existingID, existingOpenID string
	err = tx.QueryRowContext(ctx, `SELECT id,open_id FROM users WHERE phone=? AND id<>? LIMIT 1 FOR UPDATE`, phone, userID).Scan(&existingID, &existingOpenID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if !strings.HasPrefix(existingOpenID, "temporary:") {
			return domain.ErrPhoneInUse
		}
		if err := mergeTemporaryUser(ctx, tx, existingID, userID, phone); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET phone=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, phone, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rental_requests SET phone=? WHERE user_id=? AND phone=''`, phone, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func mergeTemporaryUser(ctx context.Context, tx *sql.Tx, temporaryID, userID, phone string) error {
	var role, status, createdBy string
	adminErr := tx.QueryRowContext(ctx, `SELECT role,status,created_by FROM admin_users WHERE user_id=? FOR UPDATE`, temporaryID).Scan(&role, &status, &createdBy)
	if adminErr != nil && !errors.Is(adminErr, sql.ErrNoRows) {
		return adminErr
	}
	if adminErr == nil {
		var currentStatus string
		currentErr := tx.QueryRowContext(ctx, `SELECT status FROM admin_users WHERE user_id=? FOR UPDATE`, userID).Scan(&currentStatus)
		switch {
		case errors.Is(currentErr, sql.ErrNoRows):
			if _, err := tx.ExecContext(ctx, `UPDATE admin_users SET user_id=?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?`, userID, temporaryID); err != nil {
				return err
			}
		case currentErr != nil:
			return currentErr
		default:
			if currentStatus == "active" || status == "active" {
				status = "active"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE admin_users SET role=?,status=?,created_by=?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?`, role, status, createdBy, userID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM admin_users WHERE user_id=?`, temporaryID); err != nil {
				return err
			}
		}
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT IGNORE INTO property_favorites (user_id,property_id,created_at) SELECT ?,property_id,created_at FROM property_favorites WHERE user_id=?`, []any{userID, temporaryID}},
		{`DELETE FROM property_favorites WHERE user_id=?`, []any{temporaryID}},
		{`INSERT INTO user_footprints (user_id,property_id,viewed_at,view_count) SELECT ?,property_id,viewed_at,view_count FROM user_footprints WHERE user_id=? ON DUPLICATE KEY UPDATE viewed_at=GREATEST(viewed_at,VALUES(viewed_at)),view_count=view_count+VALUES(view_count)`, []any{userID, temporaryID}},
		{`DELETE FROM user_footprints WHERE user_id=?`, []any{temporaryID}},
		{`UPDATE rental_requests SET user_id=? WHERE user_id=?`, []any{userID, temporaryID}},
		{`UPDATE properties SET owner_id=? WHERE owner_id=?`, []any{userID, temporaryID}},
		{`UPDATE property_status_history SET operator_id=? WHERE operator_id=?`, []any{userID, temporaryID}},
		{`UPDATE operation_logs SET operator_id=? WHERE operator_id=?`, []any{userID, temporaryID}},
		{`UPDATE users target JOIN users temporary ON temporary.id=? SET target.nickname=IF(target.nickname='',temporary.nickname,target.nickname),target.avatar_url=IF(target.avatar_url='',temporary.avatar_url,target.avatar_url) WHERE target.id=?`, []any{temporaryID, userID}},
		{`UPDATE temporary_access_codes SET status='disabled',updated_at=CURRENT_TIMESTAMP WHERE phone=?`, []any{phone}},
		{`DELETE FROM users WHERE id=?`, []any{temporaryID}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQL) ListAdminUsers(ctx context.Context) ([]domain.AdminUser, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.user_id,u.nickname,u.avatar_url,u.phone,a.role,a.status,a.created_by,a.created_at FROM admin_users a JOIN users u ON u.id=a.user_id ORDER BY a.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AdminUser, 0)
	for rows.Next() {
		var admin domain.AdminUser
		if err := rows.Scan(&admin.ID, &admin.UserID, &admin.Nickname, &admin.AvatarURL, &admin.Phone, &admin.Role, &admin.Status, &admin.CreatedBy, &admin.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, admin)
	}
	return items, rows.Err()
}

func (s *MySQL) AssignAdminByPhone(ctx context.Context, phone, createdBy string) (domain.AdminUser, error) {
	var userID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE phone=? LIMIT 1`, phone).Scan(&userID); err != nil {
		return domain.AdminUser{}, err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_users (user_id,role,status,created_by) VALUES (?,'admin','active',?) ON DUPLICATE KEY UPDATE status='active',updated_at=CURRENT_TIMESTAMP`, userID, createdBy)
	if err != nil {
		return domain.AdminUser{}, err
	}
	var admin domain.AdminUser
	err = s.db.QueryRowContext(ctx, `SELECT a.id,a.user_id,u.nickname,u.avatar_url,u.phone,a.role,a.status,a.created_by,a.created_at FROM admin_users a JOIN users u ON u.id=a.user_id WHERE a.user_id=?`, userID).Scan(&admin.ID, &admin.UserID, &admin.Nickname, &admin.AvatarURL, &admin.Phone, &admin.Role, &admin.Status, &admin.CreatedBy, &admin.CreatedAt)
	return admin, err
}

func (s *MySQL) SetAdminStatus(ctx context.Context, userID, status string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE admin_users SET status=?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?`, status, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *MySQL) DeleteAdminUser(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM admin_users WHERE user_id=?`, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *MySQL) IsAdminUser(ctx context.Context, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users WHERE user_id=? AND status='active'`, userID).Scan(&count)
	return count > 0, err
}

func (s *MySQL) ListTemporaryAccess(ctx context.Context) ([]domain.TemporaryAccess, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,phone,COALESCE(access_code,''),status,expires_at,created_by,created_at FROM temporary_access_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TemporaryAccess, 0)
	for rows.Next() {
		var item domain.TemporaryAccess
		if err := rows.Scan(&item.ID, &item.Phone, &item.AccessCode, &item.Status, &item.ExpiresAt, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) UpsertTemporaryAccess(ctx context.Context, phone, codeHash, accessCode, createdBy string, expiresAt time.Time) (domain.TemporaryAccess, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO temporary_access_codes (phone,code_hash,access_code,status,expires_at,created_by) VALUES (?,?,?,'active',?,?) ON DUPLICATE KEY UPDATE code_hash=VALUES(code_hash),access_code=VALUES(access_code),status='active',expires_at=VALUES(expires_at),created_by=VALUES(created_by),updated_at=CURRENT_TIMESTAMP`, phone, codeHash, accessCode, expiresAt, createdBy)
	if err != nil {
		return domain.TemporaryAccess{}, err
	}
	var item domain.TemporaryAccess
	err = s.db.QueryRowContext(ctx, `SELECT id,phone,COALESCE(access_code,''),status,expires_at,created_by,created_at FROM temporary_access_codes WHERE phone=?`, phone).Scan(&item.ID, &item.Phone, &item.AccessCode, &item.Status, &item.ExpiresAt, &item.CreatedBy, &item.CreatedAt)
	return item, err
}

func (s *MySQL) SetTemporaryAccessStatus(ctx context.Context, phone, status string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE temporary_access_codes SET status=?,updated_at=CURRENT_TIMESTAMP WHERE phone=?`, status, phone)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *MySQL) FindBoundWechatUserByPhone(ctx context.Context, phone string) (domain.User, error) {
	var user domain.User
	err := s.db.QueryRowContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at FROM users WHERE phone=? AND phone<>'' AND open_id NOT LIKE 'temporary:%' LIMIT 1`, phone).Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func (s *MySQL) LoginTemporaryUser(ctx context.Context, phone, codeHash string, now time.Time) (domain.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	var accessID uint64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM temporary_access_codes WHERE phone=? AND code_hash=? AND status='active' AND expires_at>? FOR UPDATE`, phone, codeHash, now).Scan(&accessID); err != nil {
		return domain.User{}, err
	}
	var user domain.User
	err = tx.QueryRowContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at FROM users WHERE phone=? AND phone<>'' AND open_id NOT LIKE 'temporary:%' LIMIT 1 FOR UPDATE`, phone).Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return domain.User{}, err
	} else if _, err := tx.ExecContext(ctx, `UPDATE users SET status='active',updated_at=CURRENT_TIMESTAMP WHERE id=?`, user.ID); err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *MySQL) GetAppConfig(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.QueryRowContext(ctx, `SELECT config_value FROM app_configs WHERE config_key=?`, key).Scan(&value)
	return value, err
}

func (s *MySQL) UpsertAppConfig(ctx context.Context, key string, value []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO app_configs (config_key,config_value) VALUES (?,?) ON DUPLICATE KEY UPDATE config_value=VALUES(config_value),updated_at=CURRENT_TIMESTAMP`, key, value)
	return err
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
