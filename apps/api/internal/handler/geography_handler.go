package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type GeographyHandler struct {
	geoService *service.GeographyService
}

func NewGeographyHandler(geoService *service.GeographyService) *GeographyHandler {
	return &GeographyHandler{geoService: geoService}
}

func (h *GeographyHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	zones, err := h.geoService.ListZones(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, zones)
}

func (h *GeographyHandler) ListStates(w http.ResponseWriter, r *http.Request) {
	states, err := h.geoService.ListStates(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, states)
}

func (h *GeographyHandler) ListLGAsByState(w http.ResponseWriter, r *http.Request) {
	stateID := chi.URLParam(r, "id")
	if stateID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_STATE_ID", "State ID parameter required", nil)
		return
	}

	lgas, err := h.geoService.ListLGAsByState(r.Context(), stateID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, lgas)
}
