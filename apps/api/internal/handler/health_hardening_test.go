package handler_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"ratels-api/internal/handler"
	"ratels-api/internal/repository"
	"testing"
)

func TestReadinessFailsWhenDatabaseUnavailable(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres@127.0.0.1:1/unavailable?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	h := handler.NewHealthHandler(repository.NewRepository(pool))
	rec := httptest.NewRecorder()
	h.HealthCheck(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable database reported ready: %d %s", rec.Code, rec.Body.String())
	}
}
