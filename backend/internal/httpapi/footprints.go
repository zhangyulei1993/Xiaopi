package httpapi

import (
	"context"
	"net/http"
	"strings"

	"xiaopi/backend/internal/domain"
)

type footprintStore interface {
	RecordPropertyFootprint(context.Context, string, string) error
	ListPropertyFootprints(context.Context, string) ([]domain.Footprint, error)
	ClearPropertyFootprints(context.Context, string) error
}

func (s *Server) footprints() (footprintStore, bool) {
	store, ok := s.store.(footprintStore)
	return store, ok
}

func (s *Server) recordFootprint(w http.ResponseWriter, r *http.Request) {
	propertyID := strings.TrimSpace(r.PathValue("propertyID"))
	store, ok := s.footprints()
	if !ok || propertyID == "" {
		writeError(w, http.StatusBadRequest, "invalid footprint")
		return
	}
	if err := store.RecordPropertyFootprint(r.Context(), claimsFromContext(r.Context()).Subject, propertyID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save footprint")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func (s *Server) listFootprints(w http.ResponseWriter, r *http.Request) {
	store, ok := s.footprints()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "footprints unavailable")
		return
	}
	items, err := store.ListPropertyFootprints(r.Context(), claimsFromContext(r.Context()).Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list footprints")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) clearFootprints(w http.ResponseWriter, r *http.Request) {
	store, ok := s.footprints()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "footprints unavailable")
		return
	}
	if err := store.ClearPropertyFootprints(r.Context(), claimsFromContext(r.Context()).Subject); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear footprints")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}
