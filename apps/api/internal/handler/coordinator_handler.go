package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"ratels-api/internal/service"
	"strconv"
	"time"
)

type CoordinatorHandler struct{ service *service.CoordinatorService }

func NewCoordinatorHandler(s *service.CoordinatorService) *CoordinatorHandler {
	return &CoordinatorHandler{s}
}
func (h *CoordinatorHandler) Routes(r chi.Router) {
	r.Use(RequireAuth)
	r.Get("/dashboard", h.Report)
	r.Get("/reports", h.Report)
	r.Get("/reports/export", h.Report)
	r.Get("/members", h.Members)
	r.Patch("/members/{id}/profile", h.Correct)
	r.Get("/events", h.Events)
}
func coordinatorError(w http.ResponseWriter, err error) {
	code := 400
	if errors.Is(err, service.ErrScopeUnauthorized) || errors.Is(err, service.ErrCoordinatorUnauthorized) || errors.Is(err, service.ErrMemberCorrection) {
		code = 403
	}
	ErrorJSON(w, code, "COORDINATOR_WORKFLOW_ERROR", err.Error(), nil)
}
func (h *CoordinatorHandler) Members(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.service.Members(r.Context(), GetCurrentUser(r.Context()), r.URL.Query().Get("search"), page, limit)
	if err != nil {
		coordinatorError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *CoordinatorHandler) Correct(w http.ResponseWriter, r *http.Request) {
	var in service.MemberCorrection
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		coordinatorError(w, err)
		return
	}
	if err := h.service.CorrectMember(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "id"), in); err != nil {
		coordinatorError(w, err)
		return
	}
	JSON(w, 200, map[string]string{"status": "updated"})
}
func (h *CoordinatorHandler) Events(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.Events(r.Context(), GetCurrentUser(r.Context()), r.URL.Query().Get("state_id"))
	if err != nil {
		coordinatorError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *CoordinatorHandler) Report(w http.ResponseWriter, r *http.Request) {
	var from, until time.Time
	for key, dest := range map[string]*time.Time{"from": &from, "until": &until} {
		if raw := r.URL.Query().Get(key); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				coordinatorError(w, err)
				return
			}
			*dest = parsed
		}
	}
	out, err := h.service.Report(r.Context(), GetCurrentUser(r.Context()), from, until)
	if err != nil {
		coordinatorError(w, err)
		return
	}
	if r.URL.Path[len(r.URL.Path)-7:] == "/export" {
		if err := h.service.AuditReportExport(r.Context(), GetCurrentUser(r.Context()), out.From, out.Until); err != nil {
			coordinatorError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="coordinator-operational-report.csv"`)
		writer := csv.NewWriter(w)
		defer writer.Flush()
		_ = writer.Write([]string{"state", "lga", "members", "events", "verified_attendance"})
		for _, row := range out.Geography {
			_ = writer.Write([]string{csvSafe(row["state_name"].(string)), csvSafe(row["lga_name"].(string)), strconv.FormatInt(row["members"].(int64), 10), strconv.FormatInt(row["events"].(int64), 10), strconv.FormatInt(row["verified_attendance"].(int64), 10)})
		}
		return
	}
	JSON(w, 200, out)
}
func csvSafe(s string) string {
	if len(s) > 0 && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@') {
		return "'" + s
	}
	return s
}
