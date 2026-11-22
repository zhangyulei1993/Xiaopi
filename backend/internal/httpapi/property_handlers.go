package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
	"xiaopi/backend/internal/domain"
)

func withoutMaterials(items []domain.Property) []domain.Property {
	result := make([]domain.Property, 0, len(items))
	for _, item := range items {
		if !item.IsMaterial {
			result = append(result, item)
		}
	}
	return result
}

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	showAll := false
	if claims, ok := s.optionalClaims(r); ok && s.effectiveRole(r.Context(), claims) == roleAdmin {
		showAll = true
	}
	if s.store != nil {
		items, err := s.store.ListProperties(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		if showAll {
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": withoutMaterials(items)})
			return
		}
		config, err := s.loadPublicConfig(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load public config")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": publicProperties(items, config)})
		return
	}
	if showAll {
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": withoutMaterials(s.properties)})
		return
	}
	config, err := s.loadPublicConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load public config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": publicProperties(s.properties, config)})
}

func publicDoorplateLabel(item domain.Property) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(item.BuildingNo); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(item.UnitNo); value != "" {
		if !strings.Contains(value, "单元") {
			value += "单元"
		}
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(item.RoomNo); value != "" {
		if !strings.Contains(value, "室") {
			value += "室"
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "")
}

// publicListingTitle deliberately uses the community as the public main title.
// Listing attributes are configurable badges, not a long title assembled from
// precise address facts. This also prevents old composed titles from leaking a
// doorplate when the badge is disabled.
func publicListingTitle(item domain.Property) string {
	base := ""
	if community := strings.TrimSpace(item.Community); community != "" {
		base = community
	}
	title := strings.TrimSpace(item.Title)
	building := strings.TrimSpace(item.BuildingNo)
	unit := strings.TrimSpace(item.UnitNo)
	room := strings.TrimSpace(item.RoomNo)
	// Older editor builds occasionally persisted duplicate suffixes (for
	// example "2单元单元1803室室"). Remove both the canonical label and those
	// historic variants before returning a public title.
	variants := []string{publicDoorplateLabel(item)}
	if building != "" && unit != "" && room != "" {
		variants = append(variants,
			building+unit+room,
			building+unit+"单元"+room+"室",
			building+strings.TrimSuffix(unit, "单元")+"单元"+strings.TrimSuffix(room, "室")+"室",
		)
	}
	for _, doorplate := range variants {
		if doorplate != "" {
			title = strings.ReplaceAll(title, doorplate, "")
		}
	}
	title = strings.Trim(title, " ·・-—")
	for strings.Contains(title, "· ·") || strings.Contains(title, "··") {
		title = strings.ReplaceAll(title, "· ·", "·")
		title = strings.ReplaceAll(title, "··", "·")
	}
	if base == "" && title != "" {
		base = title
	}
	if base == "" {
		base = "房源"
	}
	if phrase := strings.TrimSpace(item.MarketingPhrase); phrase != "" {
		return base + " · " + phrase
	}
	return base
}

func publicProperties(items []domain.Property, config domain.PublicConfig) []publicProperty {
	allowedTags := make(map[string]bool, len(config.ListingTitle.EnabledFields))
	for _, field := range config.ListingTitle.EnabledFields {
		allowedTags[field] = true
	}
	result := make([]publicProperty, 0, len(items))
	for _, item := range items {
		if item.IsMaterial || item.Status != domain.PropertyAvailable {
			continue
		}
		property := publicProperty{
			ID: item.ID, Title: publicListingTitle(item), MarketingPhrase: item.MarketingPhrase, Community: item.Community, Location: item.Location, AreaLabel: item.AreaLabel, CoverFocusX: item.CoverFocusX, CoverFocusY: item.CoverFocusY,
			Latitude: item.Latitude, Longitude: item.Longitude, Layout: item.Layout, ActualLayout: item.ActualLayout, WholeLayout: item.WholeLayout,
			RoomSequence: item.RoomSequence,
			RentMonthly:  item.RentMonthly, FeeText: item.FeeText, RentType: item.RentType, ListingSource: item.ListingSource, RemainingLeaseMonths: item.RemainingLeaseMonths, TransferDeposit: item.TransferDeposit,
			AvailableDate: item.AvailableDate, DepositPayment: item.DepositPayment,
			UtilitiesText: item.UtilitiesText, AreaSqm: item.AreaSqm, ActualAreaSqm: item.ActualAreaSqm, WholeAreaSqm: item.WholeAreaSqm, FloorInfo: item.FloorInfo,
			Orientation: item.Orientation, Elevator: item.Elevator, MetroMinutes: item.MetroMinutes,
			VerificationDate: item.VerificationDate, RulesText: item.RulesText,
			Description: item.Description, Status: item.Status, Media: item.Media,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
		tags := item.DisplayTags
		// Existing records predate per-listing tag selection. Preserve the prior
		// configuration-driven presentation until an operator edits the listing.
		if tags == nil {
			tags = config.ListingTitle.EnabledFields
		}
		for _, tag := range tags {
			if tag != "community" && allowedTags[tag] {
				property.DisplayTags = append(property.DisplayTags, tag)
			}
		}
		if allowedTags["unit"] && containsString(property.DisplayTags, "unit") {
			property.DoorplateLabel = publicDoorplateLabel(item)
		}
		result = append(result, property)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Server) isAvailableProperty(ctx context.Context, propertyID string) bool {
	item, ok := s.findProperty(ctx, propertyID)
	return ok && item.Status == domain.PropertyAvailable
}

func (s *Server) findProperty(ctx context.Context, propertyID string) (domain.Property, bool) {
	var items []domain.Property
	if s.store != nil {
		loaded, err := s.store.ListProperties(ctx)
		if err != nil {
			return domain.Property{}, false
		}
		items = loaded
	} else {
		items = s.properties
	}
	for _, item := range items {
		if item.ID == propertyID {
			return item, true
		}
	}
	return domain.Property{}, false
}

// normalizeCoverFocus keeps the display rule deliberately simple: a focal
// point is optional and coarse-grained, so one original asset can safely feed
// both horizontal list cards and vertical masonry cards without extra uploads.
func normalizeCoverFocus(value int) int {
	if value <= 24 {
		return 0
	}
	if value >= 76 {
		return 100
	}
	return 50
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var p domain.Property
	if err := decodeJSON(w, r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid property"})
		return
	}
	p.Title, p.Community, p.MarketingPhrase = strings.TrimSpace(p.Title), strings.TrimSpace(p.Community), strings.TrimSpace(p.MarketingPhrase)
	canonicalCommunity, communityErr := resolveKnownCommunity(r.Context(), s.store, p.Community)
	if communityErr != nil {
		writeError(w, http.StatusInternalServerError, "读取小区库失败，房源未保存")
		return
	}
	p.Community = canonicalCommunity
	p.CoverFocusX, p.CoverFocusY = normalizeCoverFocus(p.CoverFocusX), normalizeCoverFocus(p.CoverFocusY)
	if utf8.RuneCountInString(p.MarketingPhrase) > 16 {
		writeError(w, http.StatusBadRequest, "marketingPhrase must not exceed 16 characters")
		return
	}
	if p.Title == "" || p.Community == "" || p.RentMonthly < 0 {
		writeError(w, http.StatusBadRequest, "title, community and a non-negative rentMonthly are required")
		return
	}
	propertyID, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create property id")
		return
	}
	p.ID = propertyID
	p.Status = domain.PropertyDraft
	claims := claimsFromContext(r.Context())
	if claims.Role == roleUser {
		p.OwnerID = claims.Subject
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	if s.store != nil {
		if err := s.store.CreateProperty(r.Context(), p); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	} else {
		s.properties = append(s.properties, p)
	}
	if err := s.ensureDoorplateCollectionPlan(r.Context(), p.Community, communityLocation{Address: p.Location, Latitude: p.Latitude, Longitude: p.Longitude}); err != nil {
		log.Printf("save community catalog data for %q: %v", p.Community, err)
	}
	s.logOperation(r.Context(), claims.Subject, "property", p.ID, "录入房源草稿", fmt.Sprintf("%s · %s", p.Community, p.Title))
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": p})
}

func (s *Server) updateProperty(w http.ResponseWriter, r *http.Request) {
	var p domain.Property
	if err := decodeJSON(w, r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid property payload"})
		return
	}
	p.Title, p.Community, p.MarketingPhrase = strings.TrimSpace(p.Title), strings.TrimSpace(p.Community), strings.TrimSpace(p.MarketingPhrase)
	canonicalCommunity, communityErr := resolveKnownCommunity(r.Context(), s.store, p.Community)
	if communityErr != nil {
		writeError(w, http.StatusInternalServerError, "读取小区库失败，房源未保存")
		return
	}
	p.Community = canonicalCommunity
	p.CoverFocusX, p.CoverFocusY = normalizeCoverFocus(p.CoverFocusX), normalizeCoverFocus(p.CoverFocusY)
	missing := make([]string, 0, 5)
	if strings.TrimSpace(p.ID) == "" {
		missing = append(missing, "id")
	}
	if strings.TrimSpace(p.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(p.Community) == "" {
		missing = append(missing, "community")
	}
	if p.RentMonthly < 0 {
		missing = append(missing, "non-negative rentMonthly")
	}
	if utf8.RuneCountInString(p.MarketingPhrase) > 16 {
		missing = append(missing, "marketingPhrase must not exceed 16 characters")
	}
	if len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "missing or invalid: " + strings.Join(missing, ", ")})
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	if err := s.store.UpdateProperty(r.Context(), p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	if err := s.ensureDoorplateCollectionPlan(r.Context(), p.Community, communityLocation{Address: p.Location, Latitude: p.Latitude, Longitude: p.Longitude}); err != nil {
		log.Printf("save community catalog data for %q: %v", p.Community, err)
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "property", p.ID, "编辑房源信息", fmt.Sprintf("%s · %s", p.Community, p.Title))
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": p})
}

// deleteProperty removes a listing after the administrator has confirmed it in
// the client. The store keeps customer appointment records but detaches them
// from the deleted listing, so operational follow-up is not lost.
func (s *Server) deleteProperty(w http.ResponseWriter, r *http.Request) {
	propertyID := strings.TrimSpace(r.PathValue("propertyID"))
	if propertyID == "" || s.store == nil {
		writeError(w, http.StatusBadRequest, "propertyId is required")
		return
	}
	items, err := s.store.ListProperties(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load property")
		return
	}
	var property *domain.Property
	for index := range items {
		if items[index].ID == propertyID {
			property = &items[index]
			break
		}
	}
	if property == nil {
		writeError(w, http.StatusNotFound, "房源不存在")
		return
	}
	if err := s.store.DeleteProperty(r.Context(), propertyID); err != nil {
		writeError(w, http.StatusInternalServerError, "删除房源失败")
		return
	}
	for _, media := range property.Media {
		s.removeStoredUpload(media.URL)
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "property", propertyID, "删除房源", fmt.Sprintf("%s（删除前状态：%s）", property.Title, property.Status))
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}
