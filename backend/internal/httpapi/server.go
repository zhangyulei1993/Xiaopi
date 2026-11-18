package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
	"xiaopi/backend/internal/domain"
)

type Store interface {
	ListProperties(context.Context) ([]domain.Property, error)
	CreateProperty(context.Context, domain.Property) error
	UpdateProperty(context.Context, domain.Property) error
	DeleteProperty(context.Context, string) error
	ListBuildingRules(context.Context, string) ([]domain.BuildingRule, error)
	UpsertBuildingRule(context.Context, domain.BuildingRule) error
	ListPropertyLeads(context.Context) ([]domain.PropertyLead, error)
	CreatePropertyLead(context.Context, domain.PropertyLead) error
	UpdatePropertyLeadFollowUp(context.Context, string, string, string) error
	UpdatePropertyStatus(context.Context, string, string, string, string) error
	ConvertLeadToProperty(context.Context, string, domain.Property) error
	AddPropertyMedia(context.Context, domain.PropertyMedia) error
	ReplacePropertyMedia(context.Context, string, []string) error
	UpsertUser(context.Context, domain.User) error
	ListRentalRequests(context.Context) ([]domain.RentalRequest, error)
	ListRentalRequestsByUser(context.Context, string) ([]domain.RentalRequest, error)
	FindRecentAppointment(context.Context, string, string, time.Time) (*domain.RentalRequest, error)
	CreateRentalRequest(context.Context, domain.RentalRequest) error
	UpdateRentalRequestFollowUp(context.Context, string, string, string) error
	ListContactEvents(context.Context) ([]domain.ContactEvent, error)
	CreateContactEvent(context.Context, domain.ContactEvent) (bool, error)
	CreateOperationLog(context.Context, domain.OperationLog) error
	ListOperationLogs(context.Context, int) ([]domain.OperationLog, error)
	ListFavoritePropertyIDs(context.Context, string) ([]string, error)
	SetPropertyFavorite(context.Context, string, string, bool) error
	ListFavoriteContacts(context.Context) ([]domain.FavoriteContact, error)
}

type appConfigStore interface {
	GetAppConfig(context.Context, string) ([]byte, error)
	UpsertAppConfig(context.Context, string, []byte) error
}

const publicConfigKey = "miniapp_public"

type Server struct {
	properties    []domain.Property
	store         Store
	config        Config
	tokens        tokenManager
	httpClient    *http.Client
	staticMapMu   sync.Mutex
	staticMaps    map[string]staticMapCacheEntry
	doorplateMu   sync.Mutex
	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt
	appointmentMu sync.Mutex
}

type staticMapCacheEntry struct {
	Body        []byte
	ContentType string
	ExpiresAt   time.Time
}

func NewServer(dataStore Store) *Server { return NewServerWithConfig(dataStore, ConfigFromEnv()) }

func NewServerWithConfig(dataStore Store, config Config) *Server {
	return &Server{
		store:         dataStore,
		config:        config,
		tokens:        newTokenManager(config),
		httpClient:    newOutboundHTTPClient(5 * time.Second),
		staticMaps:    map[string]staticMapCacheEntry{},
		loginAttempts: map[string]loginAttempt{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/uploads/", staticMediaCache(http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads")))))
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/v1/admin/login", s.adminLogin)
	mux.HandleFunc("GET /api/v1/admin/web-accounts", s.manageWebAdmins)
	mux.HandleFunc("POST /api/v1/admin/web-accounts", s.manageWebAdmins)
	mux.HandleFunc("PATCH /api/v1/admin/web-accounts/{username}", s.manageWebAdmins)
	mux.HandleFunc("DELETE /api/v1/admin/web-accounts/{username}", s.manageWebAdmins)
	mux.HandleFunc("POST /api/v1/admin/web-password", s.manageWebAdmins)
	mux.HandleFunc("POST /api/v1/auth/wechat/login", s.wechatLogin)
	mux.HandleFunc("POST /api/v1/auth/wechat/phone", s.requireRoles(s.bindWechatPhone, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/auth/profile", s.requireRoles(s.getUserProfile, roleUser, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/auth/profile", s.requireRoles(s.updateUserProfile, roleUser, roleAdmin))
	mux.HandleFunc("POST /api/v1/auth/avatar", s.requireRoles(s.uploadUserAvatar, roleUser, roleAdmin))
	mux.HandleFunc("POST /api/v1/auth/temporary/login", s.temporaryLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.requireRoles(s.refreshAuth, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/subscription-templates", s.subscriptionTemplates)
	mux.HandleFunc("POST /api/v1/message-subscriptions", s.requireRoles(s.registerMessageSubscription, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/users", s.requireRoles(s.listUsers, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/admin-users", s.requireRoles(s.listAdminUsers, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/admin-users", s.requireRoles(s.assignAdminUser, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/admin-users/status", s.requireRoles(s.updateAdminUserStatus, roleAdmin))
	mux.HandleFunc("DELETE /api/v1/admin/admin-users/{userID}", s.requireRoles(s.deleteAdminUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/temporary-access", s.requireRoles(s.listTemporaryAccess, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/temporary-access", s.requireRoles(s.createTemporaryAccess, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/temporary-access/status", s.requireRoles(s.updateTemporaryAccessStatus, roleAdmin))
	mux.HandleFunc("GET /api/v1/config/public", s.publicConfig)
	mux.HandleFunc("GET /api/v1/admin/config/state", s.requireRoles(s.configState, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/config/history", s.requireRoles(s.configHistory, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/config/publish", s.requireRoles(s.publishConfig, roleAdmin))
	mux.HandleFunc("PUT /api/v1/admin/config/public", s.requireRoles(s.updatePublicConfig, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/location/search", s.requireRoles(s.searchLocations, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/doorplate-plans", s.requireRoles(s.doorplatePlans, roleAdmin))
	mux.HandleFunc("PUT /api/v1/admin/doorplate-plans", s.requireRoles(s.doorplatePlans, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/community-locations", s.requireRoles(s.communityLocations, roleAdmin))
	mux.HandleFunc("PUT /api/v1/admin/community-locations", s.requireRoles(s.communityLocations, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/doorplates", s.requireRoles(s.doorplates, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/doorplates", s.requireRoles(s.doorplates, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/building-rules", s.requireRoles(s.listBuildingRules, roleAdmin))
	mux.HandleFunc("PUT /api/v1/admin/building-rules", s.requireRoles(s.upsertBuildingRule, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/location/static", s.requireRoles(s.staticLocationMap, roleAdmin))
	mux.HandleFunc("GET /api/v1/properties", s.listProperties)
	mux.HandleFunc("POST /api/v1/properties", s.requireRoles(s.createProperty, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/update", s.requireRoles(s.updateProperty, roleAdmin))
	mux.HandleFunc("DELETE /api/v1/properties/{propertyID}", s.requireRoles(s.deleteProperty, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/status", s.requireRoles(s.updatePropertyStatus, roleAdmin))
	mux.HandleFunc("POST /api/v1/property-leads/convert", s.requireRoles(s.convertPropertyLead, roleAdmin))
	mux.HandleFunc("POST /api/v1/media/upload", s.requireRoles(s.uploadMedia, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/media", s.requireRoles(s.replacePropertyMedia, roleAdmin))
	mux.HandleFunc("GET /api/v1/property-leads", s.requireRoles(s.listPropertyLeads, roleAdmin))
	mux.HandleFunc("POST /api/v1/property-leads", s.createPropertyLead)
	mux.HandleFunc("PATCH /api/v1/property-leads/follow-up", s.requireRoles(s.updatePropertyLeadFollowUp, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/rental-requests", s.requireRoles(s.listRentalRequests, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/rental-requests/follow-up", s.requireRoles(s.updateRentalRequestFollowUp, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/contact-events", s.requireRoles(s.listContactEvents, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/operation-logs", s.requireRoles(s.listOperationLogs, roleAdmin))
	mux.HandleFunc("POST /api/v1/rental-requests", s.requireRoles(s.createRentalRequest, roleUser))
	mux.HandleFunc("GET /api/v1/rental-requests/mine", s.requireRoles(s.listMyRentalRequests, roleUser))
	mux.HandleFunc("POST /api/v1/contact-events", s.requireRoles(s.createContactEvent, roleUser))
	mux.HandleFunc("GET /api/v1/favorites", s.requireRoles(s.listFavorites, roleUser))
	mux.HandleFunc("PUT /api/v1/favorites/{propertyID}", s.requireRoles(s.favoriteProperty, roleUser))
	mux.HandleFunc("DELETE /api/v1/favorites/{propertyID}", s.requireRoles(s.unfavoriteProperty, roleUser))
	mux.HandleFunc("GET /api/v1/admin/favorite-contacts", s.requireRoles(s.listFavoriteContacts, roleAdmin))
	mux.HandleFunc("POST /api/v1/footprints/{propertyID}", s.requireRoles(s.recordFootprint, roleUser))
	mux.HandleFunc("GET /api/v1/footprints", s.requireRoles(s.listFootprints, roleUser))
	mux.HandleFunc("DELETE /api/v1/footprints", s.requireRoles(s.clearFootprints, roleUser))
	return logging(cors(mux))
}

// Uploaded filenames are content-addressed by a timestamp. Long browser/CDN caching
// is therefore safe and avoids re-downloading the same listing media on every visit.
func staticMediaCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logOperation(ctx context.Context, operator, targetType, targetID, action, detail string) {
	if s.store == nil {
		return
	}
	id, err := newID()
	if err != nil {
		return
	}
	_ = s.store.CreateOperationLog(ctx, domain.OperationLog{ID: id, OperatorID: operator, TargetType: targetType, TargetID: targetID, Action: action, Detail: detail, CreatedAt: time.Now()})
}

func validFollowUpStatus(status string) bool {
	switch status {
	case "new", "contacted", "matched", "completed", "closed", "invalid", "inspection_pending":
		return true
	}
	return false
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, If-Match")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"status": "ok"}})
}

type publicProperty struct {
	ID                   string                 `json:"id"`
	Title                string                 `json:"title"`
	MarketingPhrase      string                 `json:"marketingPhrase,omitempty"`
	Community            string                 `json:"community"`
	Location             string                 `json:"location"`
	AreaLabel            string                 `json:"areaLabel"`
	CoverFocusX          int                    `json:"coverFocusX"`
	CoverFocusY          int                    `json:"coverFocusY"`
	DoorplateLabel       string                 `json:"doorplateLabel,omitempty"`
	DisplayTags          []string               `json:"displayTags,omitempty"`
	Latitude             float64                `json:"latitude"`
	Longitude            float64                `json:"longitude"`
	Layout               string                 `json:"layout"`
	ActualLayout         string                 `json:"actualLayout"`
	WholeLayout          string                 `json:"wholeLayout"`
	RentMonthly          int64                  `json:"rentMonthly"`
	FeeText              string                 `json:"feeText"`
	RentType             string                 `json:"rentType"`
	ListingSource        string                 `json:"listingSource"`
	RemainingLeaseMonths int                    `json:"remainingLeaseMonths"`
	TransferDeposit      int64                  `json:"transferDeposit"`
	AvailableDate        string                 `json:"availableDate"`
	DepositPayment       string                 `json:"depositPayment"`
	UtilitiesText        string                 `json:"utilitiesText"`
	AreaSqm              int                    `json:"areaSqm"`
	ActualAreaSqm        int                    `json:"actualAreaSqm"`
	WholeAreaSqm         int                    `json:"wholeAreaSqm"`
	FloorInfo            string                 `json:"floorInfo"`
	Orientation          string                 `json:"orientation"`
	Elevator             string                 `json:"elevator"`
	MetroMinutes         int                    `json:"metroMinutes"`
	VerificationDate     string                 `json:"verificationDate"`
	RulesText            string                 `json:"rulesText"`
	Description          string                 `json:"description"`
	Status               domain.PropertyStatus  `json:"status"`
	Media                []domain.PropertyMedia `json:"media"`
	CreatedAt            time.Time              `json:"createdAt"`
	UpdatedAt            time.Time              `json:"updatedAt"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
