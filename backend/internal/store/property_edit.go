package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"xiaopi/backend/internal/domain"
)

func requireLegacyPropertyEdit(ctx context.Context, tx *sql.Tx, propertyID string, listingOnly bool) error {
	var status domain.PropertyStatus
	var material bool
	if err := tx.QueryRowContext(ctx, "SELECT status,is_material FROM properties WHERE id=? FOR UPDATE", propertyID).Scan(&status, &material); err != nil {
		return err
	}
	if listingOnly && material {
		return domain.ErrPropertyEditConflict
	}
	if !material && (status == domain.PropertyPublished || status == domain.PropertyAvailable) {
		return domain.ErrAtomicPropertyEditRequired
	}
	return nil
}

func (s *MySQL) CommitPropertyEdit(ctx context.Context, p domain.Property, batch string, mediaIDs []string) (int, error) {
	payload, err := json.Marshal(struct {
		Property domain.Property
		MediaIDs []string
	}{p, mediaIDs})
	if err != nil {
		return 0, err
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var revision int
	var lastBatch, lastDigest string
	var status domain.PropertyStatus
	if err := tx.QueryRowContext(ctx, "SELECT edit_revision,last_edit_commit,last_edit_digest,status FROM properties WHERE id=? AND is_material=0 FOR UPDATE", p.ID).Scan(&revision, &lastBatch, &lastDigest, &status); err != nil {
		return 0, err
	}
	if lastBatch == batch {
		if revision != p.EditRevision+1 || digest != lastDigest {
			return 0, domain.ErrPropertyEditConflict
		}
		return revision, nil
	}
	if revision != p.EditRevision {
		return 0, domain.ErrPropertyEditConflict
	}
	p.IsMaterial = false
	p.MaterialCategory = ""
	if status == domain.PropertyPublished || status == domain.PropertyAvailable {
		if err := validatePublishableProperty(p); err != nil {
			return 0, err
		}
	}
	seen := make(map[string]bool, len(mediaIDs))
	for _, id := range mediaIDs {
		if seen[id] {
			return 0, domain.ErrPropertyEditMedia
		}
		seen[id] = true
		var ownerBatch string
		err := tx.QueryRowContext(ctx, "SELECT edit_batch FROM property_media WHERE property_id=? AND id=? AND staged_discarded=FALSE", p.ID, id).Scan(&ownerBatch)
		if err != nil || (ownerBatch != "" && ownerBatch != batch) {
			return 0, domain.ErrPropertyEditMedia
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM property_media WHERE property_id=? AND edit_batch=''", p.ID)
	if err != nil {
		return 0, err
	}
	var removed []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		if !seen[id] {
			removed = append(removed, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range removed {
		if _, err := tx.ExecContext(ctx, "UPDATE property_media SET edit_batch='retired',staged_at=CURRENT_TIMESTAMP,staged_discarded=TRUE WHERE property_id=? AND id=?", p.ID, id); err != nil {
			return 0, err
		}
	}
	for index, id := range mediaIDs {
		if _, err := tx.ExecContext(ctx, "UPDATE property_media SET edit_batch='',staged_at=NULL,sort_order=? WHERE property_id=? AND id=?", index, p.ID, id); err != nil {
			return 0, err
		}
	}
	if err := updatePropertyFields(ctx, tx, p); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE properties SET last_edit_commit=?,last_edit_digest=? WHERE id=?", batch, digest, p.ID); err != nil {
		return 0, err
	}
	return revision + 1, tx.Commit()
}

var layoutRoomPattern = regexp.MustCompile(`([0-9]+|[一二两三四五六七八九])\s*室`)

func layoutRoomCount(layout string) int {
	match := layoutRoomPattern.FindStringSubmatch(layout)
	if len(match) < 2 {
		return 0
	}
	if count, err := strconv.Atoi(match[1]); err == nil {
		return count
	}
	return map[string]int{"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9}[match[1]]
}

func validatePublishableProperty(p domain.Property) error {
	if p.IsMaterial {
		return fmt.Errorf("%w: 请先将素材转为房源草稿", domain.ErrPropertyNotReady)
	}
	var missing []string
	if strings.TrimSpace(p.Community) == "" {
		missing = append(missing, "小区")
	}
	if strings.TrimSpace(p.Location) == "" || p.Latitude == 0 || p.Longitude == 0 || p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		missing = append(missing, "地址和地图位置")
	}
	if strings.TrimSpace(p.Layout) == "" || (p.RentType == "合租" && strings.TrimSpace(p.WholeLayout) == "") {
		missing = append(missing, "户型")
	}
	if p.RentType == "合租" && (p.RoomSequence < 0 || p.RoomSequence > 9 || p.RoomSequence > layoutRoomCount(p.WholeLayout)) {
		missing = append(missing, "出租房间序号不能超过整套室数")
	}
	if p.RentMonthly < 0 {
		missing = append(missing, "有效月租")
	}
	if strings.TrimSpace(p.DepositPayment) == "" {
		missing = append(missing, "押付方式")
	}
	if p.ListingSource == "转租" && p.RemainingLeaseMonths <= 0 {
		missing = append(missing, "转租剩余租期")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w：%s", domain.ErrPropertyNotReady, strings.Join(missing, "、"))
	}
	return nil
}
