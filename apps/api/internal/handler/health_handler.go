package handler

import (
	"context"
	"net/http"
	"time"

	"ratels-api/internal/repository"
)

type HealthHandler struct {
	repo *repository.Repository
}

func NewHealthHandler(repo *repository.Repository) *HealthHandler {
	return &HealthHandler{repo: repo}
}

func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "healthy"
	if err := h.repo.Pool().Ping(ctx); err != nil {
		dbStatus = "unreachable"
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"database":  dbStatus,
		"version":   "1.0.0",
	})
}
