package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

const propertySelect = `SELECT id,title,marketing_phrase,community,location,area_label,cover_focus_x,cover_focus_y,building_no,unit_no,room_no,latitude,longitude,layout,actual_layout,whole_layout,room_sequence,rent_monthly,fee_text,rent_type,listing_source,remaining_lease_months,transfer_deposit,available_date,deposit_payment,utilities_text,area_sqm,actual_area_sqm,whole_area_sqm,floor_info,orientation,elevator,metro_minutes,verification_date,rules_text,description,display_tags,is_material,material_category,converted_property_id,status,owner_id,created_at,updated_at,edit_revision FROM properties`

func (s *MySQL) ListProperties(ctx context.Context) ([]domain.Property, error) {
	return s.readProperties(ctx, "")
}

func (s *MySQL) GetProperty(ctx context.Context, id string) (domain.Property, error) {
	if id == "" {
		return domain.Property{}, sql.ErrNoRows
	}
	items, err := s.readProperties(ctx, id)
	if err != nil {
		return domain.Property{}, err
	}
	if len(items) == 0 {
		return domain.Property{}, sql.ErrNoRows
	}
	return items[0], nil
}

func (s *MySQL) readProperties(ctx context.Context, id string) ([]domain.Property, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := propertySelect + " ORDER BY created_at DESC"
	var args []any
	if id != "" {
		query = propertySelect + " WHERE id=?"
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Property, 0)
	for rows.Next() {
		var p domain.Property
		var displayTags []byte
		if err := rows.Scan(&p.ID, &p.Title, &p.MarketingPhrase, &p.Community, &p.Location, &p.AreaLabel, &p.CoverFocusX, &p.CoverFocusY, &p.BuildingNo, &p.UnitNo, &p.RoomNo, &p.Latitude, &p.Longitude, &p.Layout, &p.ActualLayout, &p.WholeLayout, &p.RoomSequence, &p.RentMonthly, &p.FeeText, &p.RentType, &p.ListingSource, &p.RemainingLeaseMonths, &p.TransferDeposit, &p.AvailableDate, &p.DepositPayment, &p.UtilitiesText, &p.AreaSqm, &p.ActualAreaSqm, &p.WholeAreaSqm, &p.FloorInfo, &p.Orientation, &p.Elevator, &p.MetroMinutes, &p.VerificationDate, &p.RulesText, &p.Description, &displayTags, &p.IsMaterial, &p.MaterialCategory, &p.ConvertedPropertyID, &p.Status, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt, &p.EditRevision); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(displayTags, &p.DisplayTags)
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		media, err := listPropertyMedia(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		byProperty := make(map[string][]domain.PropertyMedia)
		for _, entry := range media {
			byProperty[entry.PropertyID] = append(byProperty[entry.PropertyID], entry)
		}
		for i := range items {
			items[i].Media = byProperty[items[i].ID]
		}
	}
	return items, tx.Commit()
}

func listPropertyMedia(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, propertyID string) ([]domain.PropertyMedia, error) {
	query := "SELECT id,property_id,media_type,url,sort_order FROM property_media WHERE edit_batch=''"
	var args []any
	if propertyID != "" {
		query += " AND property_id=?"
		args = append(args, propertyID)
	}
	rows, err := db.QueryContext(ctx, query+" ORDER BY property_id,sort_order,id", args...)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.IsMaterial {
		if err := lockMaterialCategory(ctx, tx, p.MaterialCategory); err != nil {
			return err
		}
	}
	if err := insertProperty(ctx, tx, p); err != nil {
		return err
	}
	if err := enqueueCommunitySync(ctx, tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func insertProperty(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, p domain.Property) error {
	displayTags, _ := json.Marshal(p.DisplayTags)
	_, err := db.ExecContext(ctx, `INSERT INTO properties (id,title,marketing_phrase,community,location,area_label,cover_focus_x,cover_focus_y,building_no,unit_no,room_no,latitude,longitude,layout,actual_layout,whole_layout,room_sequence,rent_monthly,fee_text,rent_type,listing_source,remaining_lease_months,transfer_deposit,available_date,deposit_payment,utilities_text,area_sqm,actual_area_sqm,whole_area_sqm,floor_info,orientation,elevator,metro_minutes,verification_date,rules_text,description,display_tags,is_material,material_category,converted_property_id,status,owner_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Title, p.MarketingPhrase, p.Community, p.Location, p.AreaLabel, p.CoverFocusX, p.CoverFocusY, p.BuildingNo, p.UnitNo, p.RoomNo, p.Latitude, p.Longitude, p.Layout, p.ActualLayout, p.WholeLayout, p.RoomSequence, p.RentMonthly, p.FeeText, p.RentType, p.ListingSource, p.RemainingLeaseMonths, p.TransferDeposit, p.AvailableDate, p.DepositPayment, p.UtilitiesText, p.AreaSqm, p.ActualAreaSqm, p.WholeAreaSqm, p.FloorInfo, p.Orientation, p.Elevator, p.MetroMinutes, p.VerificationDate, p.RulesText, p.Description, displayTags, p.IsMaterial, p.MaterialCategory, p.ConvertedPropertyID, p.Status, p.OwnerID, p.CreatedAt, p.UpdatedAt)
	return err
}

func (s *MySQL) CommitMaterialConversion(ctx context.Context, source domain.Property, draft domain.Property) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var linked string
	var revision int
	if err := tx.QueryRowContext(ctx, "SELECT converted_property_id, edit_revision FROM properties WHERE id=? AND is_material=1 FOR UPDATE", source.ID).Scan(&linked, &revision); err != nil {
		return err
	}
	if linked != source.ConvertedPropertyID || revision != source.EditRevision {
		return domain.ErrMaterialConversionChanged
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM property_media WHERE property_id=? AND edit_batch='' ORDER BY sort_order,id FOR UPDATE", source.ID)
	if err != nil {
		return err
	}
	index := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if index >= len(source.Media) || source.Media[index].ID != id {
			rows.Close()
			return domain.ErrMaterialConversionChanged
		}
		index++
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if closeErr != nil {
		return closeErr
	}
	if index != len(source.Media) {
		return domain.ErrMaterialConversionChanged
	}
	if err := insertProperty(ctx, tx, draft); err != nil {
		return err
	}
	if err := enqueueCommunitySync(ctx, tx, draft); err != nil {
		return err
	}
	for index, media := range draft.Media {
		result, err := tx.ExecContext(ctx, "INSERT INTO property_media (id,property_id,media_type,url,sort_order) SELECT ?,?,media_type,url,? FROM property_media WHERE id=? AND property_id=?", media.ID, draft.ID, media.Sort, source.Media[index].ID, source.ID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return domain.ErrMaterialConversionChanged
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE properties SET converted_property_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", draft.ID, source.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) UpdateProperty(ctx context.Context, p domain.Property) error {
	if p.IsMaterial {
		_, err := s.updateMaterialRevision(ctx, p, false)
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireLegacyPropertyEdit(ctx, tx, p.ID, true); err != nil {
		return err
	}
	if err := updatePropertyFields(ctx, tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) UpdateMaterialRevision(ctx context.Context, p domain.Property) (int, error) {
	if p.RentType == "合租" && (p.RoomSequence < 0 || p.RoomSequence > 9 || p.RoomSequence > layoutRoomCount(p.WholeLayout)) {
		return 0, domain.ErrMaterialRoomSequence
	}
	return s.updateMaterialRevision(ctx, p, true)
}

func (s *MySQL) updateMaterialRevision(ctx context.Context, p domain.Property, checkCategory bool) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if checkCategory {
		if err := lockMaterialCategory(ctx, tx, p.MaterialCategory); err != nil {
			return 0, err
		}
	}
	var revision int
	if err := tx.QueryRowContext(ctx, "SELECT edit_revision FROM properties WHERE id=? AND is_material=1 FOR UPDATE", p.ID).Scan(&revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.ErrPropertyEditConflict
		}
		return 0, err
	}
	if revision != p.EditRevision {
		return 0, domain.ErrPropertyEditConflict
	}
	if err := updatePropertyFields(ctx, tx, p); err != nil {
		return 0, err
	}
	return revision + 1, tx.Commit()
}

func updatePropertyFields(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, p domain.Property) error {
	displayTags, _ := json.Marshal(p.DisplayTags)
	_, err := db.ExecContext(ctx, `UPDATE properties SET title=?,marketing_phrase=?,community=?,location=?,area_label=?,cover_focus_x=?,cover_focus_y=?,building_no=?,unit_no=?,room_no=?,latitude=?,longitude=?,layout=?,actual_layout=?,whole_layout=?,room_sequence=?,rent_monthly=?,fee_text=?,rent_type=?,listing_source=?,remaining_lease_months=?,transfer_deposit=?,available_date=?,deposit_payment=?,utilities_text=?,area_sqm=?,actual_area_sqm=?,whole_area_sqm=?,floor_info=?,orientation=?,elevator=?,metro_minutes=?,verification_date=?,rules_text=?,description=?,display_tags=?,is_material=?,material_category=?,edit_revision=edit_revision+1,updated_at=CURRENT_TIMESTAMP WHERE id=?`, p.Title, p.MarketingPhrase, p.Community, p.Location, p.AreaLabel, p.CoverFocusX, p.CoverFocusY, p.BuildingNo, p.UnitNo, p.RoomNo, p.Latitude, p.Longitude, p.Layout, p.ActualLayout, p.WholeLayout, p.RoomSequence, p.RentMonthly, p.FeeText, p.RentType, p.ListingSource, p.RemainingLeaseMonths, p.TransferDeposit, p.AvailableDate, p.DepositPayment, p.UtilitiesText, p.AreaSqm, p.ActualAreaSqm, p.WholeAreaSqm, p.FloorInfo, p.Orientation, p.Elevator, p.MetroMinutes, p.VerificationDate, p.RulesText, p.Description, displayTags, p.IsMaterial, p.MaterialCategory, p.ID)
	if err != nil {
		return err
	}
	return enqueueCommunitySync(ctx, db, p)
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
// transaction. Appointment records intentionally retain their property ID and
// title snapshot so the customer and operator can trace the original listing
// after it has been removed.
func (s *MySQL) DeleteProperty(ctx context.Context, propertyID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lockedID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM properties WHERE id=? FOR UPDATE", propertyID).Scan(&lockedID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE property_media SET edit_batch='retired',staged_at=CURRENT_TIMESTAMP,staged_discarded=TRUE WHERE property_id=? AND edit_batch=''`, propertyID); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_name,source_phone,community,location,layout,expected_rent_monthly,note,follow_up_note,CASE WHEN COALESCE(converted_property_id,'')<>'' THEN 'converted' ELSE status END,COALESCE(converted_property_id,''),created_at,updated_at FROM property_leads ORDER BY created_at DESC`)
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
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.user_id,r.request_type,r.property_id,r.property_title,(p.id IS NOT NULL),r.contact_name,r.phone,r.preferred_time,r.location,r.layout,r.budget_min,r.budget_max,r.move_in_date,r.note,r.follow_up_note,r.status,r.created_at,r.updated_at FROM rental_requests r LEFT JOIN properties p ON p.id=r.property_id ORDER BY r.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RentalRequest, 0)
	for rows.Next() {
		var item domain.RentalRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestType, &item.PropertyID, &item.PropertyTitle, &item.PropertyAvailable, &item.ContactName, &item.Phone, &item.PreferredTime, &item.Location, &item.Layout, &item.BudgetMin, &item.BudgetMax, &item.MoveInDate, &item.Note, &item.FollowUpNote, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) ListRentalRequestsByUser(ctx context.Context, userID string) ([]domain.RentalRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.user_id,r.request_type,r.property_id,r.property_title,COALESCE(p.status='available' AND p.is_material=FALSE,FALSE),r.contact_name,r.phone,r.preferred_time,r.location,r.layout,r.budget_min,r.budget_max,r.move_in_date,r.note,r.follow_up_note,r.status,r.created_at,r.updated_at FROM rental_requests r LEFT JOIN properties p ON p.id=r.property_id WHERE r.user_id=? ORDER BY r.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RentalRequest, 0)
	for rows.Next() {
		var item domain.RentalRequest
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestType, &item.PropertyID, &item.PropertyTitle, &item.PropertyAvailable, &item.ContactName, &item.Phone, &item.PreferredTime, &item.Location, &item.Layout, &item.BudgetMin, &item.BudgetMax, &item.MoveInDate, &item.Note, &item.FollowUpNote, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) FindRecentAppointment(ctx context.Context, userID, propertyID string, since time.Time) (*domain.RentalRequest, error) {
	return findRecentAppointment(ctx, s.db, userID, propertyID, since)
}

func findRecentAppointment(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID, propertyID string, since time.Time) (*domain.RentalRequest, error) {
	var item domain.RentalRequest
	err := db.QueryRowContext(ctx, `SELECT id,user_id,request_type,property_id,contact_name,phone,preferred_time,location,layout,budget_min,budget_max,move_in_date,note,follow_up_note,status,created_at,updated_at FROM rental_requests WHERE user_id=? AND property_id=? AND request_type='appointment' AND created_at>=? ORDER BY created_at DESC LIMIT 1`, userID, propertyID, since).Scan(&item.ID, &item.UserID, &item.RequestType, &item.PropertyID, &item.ContactName, &item.Phone, &item.PreferredTime, &item.Location, &item.Layout, &item.BudgetMin, &item.BudgetMax, &item.MoveInDate, &item.Note, &item.FollowUpNote, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *MySQL) UpdatePropertyLeadFollowUp(ctx context.Context, id, status, note string) error {
	return s.updateFollowUp(ctx, "property_leads", id, status, note)
}

func (s *MySQL) UpdateRentalRequestFollowUp(ctx context.Context, id, status, note string) error {
	return s.updateFollowUp(ctx, "rental_requests", id, status, note)
}

func (s *MySQL) updateFollowUp(ctx context.Context, table, id, status, note string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentStatus string
	statusSQL := "SELECT status FROM rental_requests WHERE id=? FOR UPDATE"
	if table == "property_leads" {
		statusSQL = "SELECT CASE WHEN COALESCE(converted_property_id,'')<>'' THEN 'converted' ELSE status END FROM property_leads WHERE id=? FOR UPDATE"
	}
	if err := tx.QueryRowContext(ctx, statusSQL, id).Scan(&currentStatus); err != nil {
		return err
	}
	if table == "property_leads" && currentStatus == string(domain.LeadConverted) {
		return domain.ErrLeadFollowUpConflict
	}
	if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET status=?,follow_up_note=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", status, note, id); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *MySQL) ListOperationLogsPage(ctx context.Context, category, startDate, endDate string, limit, offset int) ([]domain.OperationLog, int, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 4)
	switch category {
	case "property":
		conditions = append(conditions, `target_type IN ('property','material','building_rule')`)
	case "customer":
		conditions = append(conditions, `target_type IN ('lead','request')`)
	case "message":
		conditions = append(conditions, `target_type='subscription'`)
	case "all", "":
	default:
		return nil, 0, fmt.Errorf("invalid operation log category")
	}
	if startDate != "" {
		conditions = append(conditions, `created_at >= ?`)
		args = append(args, startDate+` 00:00:00`)
	}
	if endDate != "" {
		conditions = append(conditions, `created_at < DATE_ADD(?, INTERVAL 1 DAY)`)
		args = append(args, endDate+` 00:00:00`)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM operation_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,operator_id,target_type,target_id,action,detail,created_at FROM operation_logs`+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.OperationLog, 0)
	for rows.Next() {
		var item domain.OperationLog
		if err := rows.Scan(&item.ID, &item.OperatorID, &item.TargetType, &item.TargetID, &item.Action, &item.Detail, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *MySQL) CreateRentalRequest(ctx context.Context, item domain.RentalRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertRentalRequest(ctx, tx, item); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) CreateAppointmentOnce(ctx context.Context, item domain.RentalRequest) (*domain.RentalRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var status domain.PropertyStatus
	var material bool
	if err := tx.QueryRowContext(ctx, "SELECT status,is_material FROM properties WHERE id=? FOR UPDATE", item.PropertyID).Scan(&status, &material); err != nil {
		return nil, err
	}
	if status != domain.PropertyAvailable || material {
		return nil, sql.ErrNoRows
	}
	existing, err := findRecentAppointment(ctx, tx, item.UserID, item.PropertyID, item.CreatedAt.Add(-30*time.Minute))
	if err != nil || existing != nil {
		return existing, err
	}
	if err := insertRentalRequest(ctx, tx, item); err != nil {
		return nil, err
	}
	return nil, tx.Commit()
}

func insertRentalRequest(ctx context.Context, tx *sql.Tx, item domain.RentalRequest) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO rental_requests (id,user_id,request_type,property_id,property_title,contact_name,phone,preferred_time,location,layout,budget_min,budget_max,move_in_date,note,status,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.UserID, item.RequestType, item.PropertyID, item.PropertyTitle, item.ContactName, item.Phone, item.PreferredTime, item.Location, item.Layout, item.BudgetMin, item.BudgetMax, item.MoveInDate, item.Note, item.Status, item.CreatedAt)
	if err != nil {
		return err
	}
	return enqueueSubscription(ctx, tx, "request", item.ID, item)
}

const contactEventPageSQL = `SELECT e.id,e.user_id,e.property_id,e.event_type,e.source,COALESCE(u.nickname,''),COALESCE(u.phone,''),COALESCE(NULLIF(p.community,''),p.title,''),e.created_at FROM property_contact_events e LEFT JOIN users u ON u.id=e.user_id LEFT JOIN properties p ON p.id=e.property_id ORDER BY e.created_at DESC,e.id DESC LIMIT ? OFFSET ?`

func (s *MySQL) ListContactEvents(ctx context.Context) ([]domain.ContactEvent, error) {
	rows, err := s.db.QueryContext(ctx, contactEventPageSQL, 200, 0)
	return scanContactEvents(rows, err)
}

func (s *MySQL) ListContactEventsPage(ctx context.Context, limit, offset int) ([]domain.ContactEvent, int, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var total int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM property_contact_events").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, contactEventPageSQL, limit, offset)
	items, err := scanContactEvents(rows, err)
	if err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit()
}

func scanContactEvents(rows *sql.Rows, err error) ([]domain.ContactEvent, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ContactEvent, 0)
	for rows.Next() {
		var item domain.ContactEvent
		if err := rows.Scan(&item.ID, &item.UserID, &item.PropertyID, &item.Type, &item.Source, &item.Nickname, &item.Phone, &item.PropertyName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) CreateContactEvent(ctx context.Context, item domain.ContactEvent) (bool, error) {
	var exists string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM property_contact_events WHERE user_id=? AND property_id=? AND event_type=? AND created_at >= DATE_SUB(NOW(), INTERVAL 30 MINUTE) ORDER BY created_at DESC LIMIT 1`, item.UserID, item.PropertyID, item.Type).Scan(&exists)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO property_contact_events (id,user_id,property_id,event_type,source,created_at) VALUES (?,?,?,?,?,?)`, item.ID, item.UserID, item.PropertyID, item.Type, item.Source, item.CreatedAt)
	return err == nil, err
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
func (s *MySQL) ListAdminMessageRecipientsForJob(ctx context.Context, templateID string, jobID uint64) ([]domain.MessageRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id,g.user_id,u.open_id,g.area FROM message_subscription_grants g JOIN users u ON u.id=g.user_id JOIN admin_users a ON a.user_id=g.user_id AND a.status='active' WHERE g.template_id=? AND g.status='active' AND g.id=(SELECT MIN(g2.id) FROM message_subscription_grants g2 WHERE g2.user_id=g.user_id AND g2.template_id=g.template_id AND g2.status='active') AND g.id<=(SELECT grant_ceiling FROM subscription_outbox WHERE id=?) AND NOT EXISTS(SELECT 1 FROM subscription_deliveries d WHERE d.job_id=? AND d.user_id=g.user_id) ORDER BY g.created_at LIMIT 50`, templateID, jobID, jobID)
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
func (s *MySQL) ListAreaMessageRecipientsForJob(ctx context.Context, templateID string, property domain.Property, jobID uint64) ([]domain.MessageRecipient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id,g.user_id,u.open_id,g.area FROM message_subscription_grants g JOIN users u ON u.id=g.user_id WHERE g.template_id=? AND g.status='active' AND g.area<>'' AND g.id=(SELECT MIN(g2.id) FROM message_subscription_grants g2 WHERE g2.user_id=g.user_id AND g2.template_id=g.template_id AND g2.area=g.area AND g2.status='active') AND (INSTR(?,g.area)>0 OR INSTR(?,g.area)>0 OR INSTR(?,g.area)>0) AND g.id<=(SELECT grant_ceiling FROM subscription_outbox WHERE id=?) AND NOT EXISTS(SELECT 1 FROM subscription_deliveries d WHERE d.job_id=? AND d.user_id=g.user_id) ORDER BY g.created_at LIMIT 200`, templateID, property.Location, property.Community, property.Title, jobID, jobID)
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

func (s *MySQL) FindPropertyMedia(ctx context.Context, propertyID, id string) (domain.PropertyMedia, error) {
	var media domain.PropertyMedia
	err := s.db.QueryRowContext(ctx, "SELECT id,property_id,media_type,url,sort_order FROM property_media WHERE property_id=? AND id=? AND staged_discarded=FALSE", propertyID, id).Scan(&media.ID, &media.PropertyID, &media.Type, &media.URL, &media.Sort)
	return media, err
}

func (s *MySQL) AddPropertyMedia(ctx context.Context, media domain.PropertyMedia) error {
	propertyID := media.PropertyID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if media.EditBatch == "" {
		if err := requireLegacyPropertyEdit(ctx, tx, propertyID, false); err != nil {
			return err
		}
	} else {
		var lockedID string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM properties WHERE id=? FOR UPDATE", propertyID).Scan(&lockedID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO property_media (id,property_id,media_type,url,sort_order,edit_batch,staged_at) VALUES (?,?,?,?,?,?,IF(?='',NULL,CURRENT_TIMESTAMP))`, media.ID, media.PropertyID, media.Type, media.URL, media.Sort, media.EditBatch, media.EditBatch)
	if err != nil {
		return err
	}
	if media.EditBatch == "" {
		if _, err := tx.ExecContext(ctx, "UPDATE properties SET edit_revision=edit_revision+1 WHERE id=?", propertyID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplacePropertyMedia persists one complete editor ordering. IDs omitted from
// the order are removed; IDs owned by another property are rejected.
func (s *MySQL) ReplacePropertyMedia(ctx context.Context, propertyID string, mediaIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireLegacyPropertyEdit(ctx, tx, propertyID, false); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id FROM property_media WHERE property_id=? AND edit_batch='' FOR UPDATE`, propertyID)
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
			if _, err := tx.ExecContext(ctx, `UPDATE property_media SET edit_batch='retired',staged_at=CURRENT_TIMESTAMP,staged_discarded=TRUE WHERE id=? AND property_id=?`, id, propertyID); err != nil {
				return err
			}
		}
	}
	for sort, id := range mediaIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE property_media SET sort_order=? WHERE id=? AND property_id=?`, sort, id, propertyID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE properties SET edit_revision=edit_revision+1 WHERE id=?", propertyID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) UpdatePropertyStatus(ctx context.Context, propertyID, nextStatus, operatorID, note string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM properties WHERE id=? FOR UPDATE`, propertyID).Scan(&current); err != nil {
		return "", err
	}
	// The edit page can be reopened after a network retry. Repeating the target
	// status is intentionally idempotent rather than an operator-facing 409.
	if current == nextStatus {
		return "", tx.Commit()
	}
	if !domain.CanTransitionPropertyStatus(current, nextStatus) {
		return "", domain.ErrInvalidPropertyStatusTransition
	}
	if nextStatus == string(domain.PropertyPublished) || nextStatus == string(domain.PropertyAvailable) {
		var p domain.Property
		err := tx.QueryRowContext(ctx, "SELECT is_material,community,location,latitude,longitude,layout,whole_layout,room_sequence,rent_type,rent_monthly,deposit_payment,listing_source,remaining_lease_months FROM properties WHERE id=?", propertyID).Scan(&p.IsMaterial, &p.Community, &p.Location, &p.Latitude, &p.Longitude, &p.Layout, &p.WholeLayout, &p.RoomSequence, &p.RentType, &p.RentMonthly, &p.DepositPayment, &p.ListingSource, &p.RemainingLeaseMonths)
		if err != nil {
			return "", err
		}
		if err := validatePublishableProperty(p); err != nil {
			return "", err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE properties SET status=?,edit_revision=edit_revision+1,updated_at=CURRENT_TIMESTAMP WHERE id=?`, nextStatus, propertyID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO property_status_history (id,property_id,from_status,to_status,operator_id,note) VALUES (?,?,?,?,?,?)`, fmt.Sprintf("%d", time.Now().UnixNano()), propertyID, current, nextStatus, operatorID, note); err != nil {
		return "", err
	}
	if nextStatus == string(domain.PropertyAvailable) {
		if err := enqueueSubscription(ctx, tx, "property", propertyID, nil); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return current, nil
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
	result, err := tx.ExecContext(ctx, `UPDATE property_leads SET status='converted',converted_property_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND status NOT IN ('converted','closed','invalid') AND COALESCE(converted_property_id,'')=''`, p.ID, leadID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *MySQL) UpsertUser(ctx context.Context, user domain.User) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id,open_id,nickname,avatar_url,phone,status,last_login_at) VALUES (?,?,?,?,?,?,CURRENT_TIMESTAMP) ON DUPLICATE KEY UPDATE last_login_at=CURRENT_TIMESTAMP, open_id=VALUES(open_id),nickname=IF(VALUES(nickname)='',nickname,VALUES(nickname)),avatar_url=IF(VALUES(avatar_url)='',avatar_url,VALUES(avatar_url)),phone=IF(VALUES(phone)='',phone,VALUES(phone)),status=IF(status='blocked',status,IF(VALUES(status)='',status,VALUES(status))),updated_at=CURRENT_TIMESTAMP`, user.ID, user.OpenID, user.Nickname, user.AvatarURL, user.Phone, user.Status)
	return err
}

func (s *MySQL) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at,last_login_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,phone,COALESCE(access_code,''),status,UNIX_TIMESTAMP(expires_at),created_by,UNIX_TIMESTAMP(created_at) FROM temporary_access_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TemporaryAccess, 0)
	for rows.Next() {
		var item domain.TemporaryAccess
		var expiresUnix, createdUnix int64
		if err := rows.Scan(&item.ID, &item.Phone, &item.AccessCode, &item.Status, &expiresUnix, &item.CreatedBy, &createdUnix); err != nil {
			return nil, err
		}
		item.ExpiresAt, item.CreatedAt = time.Unix(expiresUnix, 0), time.Unix(createdUnix, 0)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQL) UpsertTemporaryAccess(ctx context.Context, phone, codeHash, accessCode, createdBy string, expiresAt time.Time) (domain.TemporaryAccess, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO temporary_access_codes (phone,code_hash,access_code,status,expires_at,created_by) VALUES (?,?,?,'active',FROM_UNIXTIME(?),?) ON DUPLICATE KEY UPDATE code_hash=VALUES(code_hash),access_code=VALUES(access_code),status='active',expires_at=VALUES(expires_at),created_by=VALUES(created_by),session_version=session_version+1,updated_at=CURRENT_TIMESTAMP`, phone, codeHash, accessCode, expiresAt.Unix(), createdBy)
	if err != nil {
		return domain.TemporaryAccess{}, err
	}
	var item domain.TemporaryAccess
	var expiresUnix, createdUnix int64
	err = s.db.QueryRowContext(ctx, `SELECT id,phone,COALESCE(access_code,''),status,UNIX_TIMESTAMP(expires_at),created_by,UNIX_TIMESTAMP(created_at) FROM temporary_access_codes WHERE phone=?`, phone).Scan(&item.ID, &item.Phone, &item.AccessCode, &item.Status, &expiresUnix, &item.CreatedBy, &createdUnix)
	item.ExpiresAt, item.CreatedAt = time.Unix(expiresUnix, 0), time.Unix(createdUnix, 0)
	return item, err
}

func (s *MySQL) SetTemporaryAccessStatus(ctx context.Context, phone, status string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE temporary_access_codes SET session_version=session_version+IF(status<>?,1,0),status=?,updated_at=CURRENT_TIMESTAMP WHERE phone=?`, status, status, phone)
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

func (s *MySQL) LoginTemporaryUser(ctx context.Context, phone, codeHash string, now time.Time) (domain.User, domain.TemporaryGrant, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, domain.TemporaryGrant{}, err
	}
	defer tx.Rollback()
	var grant domain.TemporaryGrant
	var expiresUnix int64
	if err := tx.QueryRowContext(ctx, `SELECT id,session_version,UNIX_TIMESTAMP(expires_at) FROM temporary_access_codes WHERE phone=? AND code_hash=? AND status='active' AND expires_at>FROM_UNIXTIME(?) FOR UPDATE`, phone, codeHash, now.Unix()).Scan(&grant.ID, &grant.Version, &expiresUnix); err != nil {
		return domain.User{}, domain.TemporaryGrant{}, err
	}
	grant.ExpiresAt = time.Unix(expiresUnix, 0)
	var user domain.User
	err = tx.QueryRowContext(ctx, `SELECT id,open_id,nickname,avatar_url,phone,status,created_at,updated_at FROM users WHERE phone=? AND phone<>'' AND open_id NOT LIKE 'temporary:%' LIMIT 1 FOR UPDATE`, phone).Scan(&user.ID, &user.OpenID, &user.Nickname, &user.AvatarURL, &user.Phone, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return domain.User{}, domain.TemporaryGrant{}, err
	} else if user.Status == domain.UserBlocked {
		return domain.User{}, domain.TemporaryGrant{}, sql.ErrNoRows
	} else if _, err := tx.ExecContext(ctx, `UPDATE users SET last_login_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=?`, user.ID); err != nil {
		return domain.User{}, domain.TemporaryGrant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, domain.TemporaryGrant{}, err
	}
	return user, grant, nil
}

func (s *MySQL) ValidateTemporarySession(ctx context.Context, userID string, id, version uint64) error {
	var matched uint64
	return s.db.QueryRowContext(ctx, "SELECT t.id FROM temporary_access_codes t JOIN users u ON u.phone=t.phone WHERE t.id=? AND t.session_version=? AND t.status='active' AND t.expires_at>NOW() AND u.id=?", id, version, userID).Scan(&matched)
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
