package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type NotificationHandler struct {
	notifService *service.NotificationService
}

func NewNotificationHandler(notifService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notifService: notifService,
	}
}

// -------------------------------------------------------------
// Member Endpoints (/me/notifications)
// -------------------------------------------------------------

// ListNotifications returns paginated notifications for current member.
func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	page := int32(1)
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}
	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = int32(l)
	}

	var unreadOnly *bool
	if val := q.Get("unread_only"); val != "" {
		b := val == "true" || val == "1"
		unreadOnly = &b
	}

	var notifType *string
	if val := q.Get("type"); val != "" {
		notifType = &val
	}

	resp, err := h.notifService.ListUserNotifications(r.Context(), user.ID, unreadOnly, notifType, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list notifications", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// GetUnreadCount returns total unread notifications count for current member.
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	count, err := h.notifService.GetUnreadCount(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve unread count", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]int64{
		"unread_count": count,
	})
}

// MarkAsRead marks a specific notification as read.
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Notification ID is required", nil)
		return
	}

	notif, err := h.notifService.MarkAsRead(r.Context(), user.ID, id)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Notification not found", nil)
		return
	}

	JSON(w, http.StatusOK, notif)
}

// MarkAllAsRead marks all notifications as read for current member.
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	err := h.notifService.MarkAllAsRead(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark all as read", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "All notifications marked as read",
	})
}

// DeleteNotification deletes a specific notification.
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Notification ID is required", nil)
		return
	}

	err := h.notifService.DeleteNotification(r.Context(), user.ID, id)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete notification", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Notification deleted",
	})
}

// -------------------------------------------------------------
// Preferences Endpoints (/me/notification-preferences)
// -------------------------------------------------------------

// GetPreferences returns notification preferences for current member.
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	prefs, err := h.notifService.GetPreferences(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve preferences", err.Error())
		return
	}

	JSON(w, http.StatusOK, prefs)
}

// UpdatePreferences updates channel and topic toggles.
func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
		return
	}

	updated, err := h.notifService.UpdatePreferences(r.Context(), user.ID, req)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update notification preferences", err.Error())
		return
	}

	JSON(w, http.StatusOK, updated)
}

// -------------------------------------------------------------
// Admin Communications Endpoints (/admin/communications)
// -------------------------------------------------------------

// GetOverviewStats returns high-level metrics for the notification outbox.
func (h *NotificationHandler) GetOverviewStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.notifService.GetOverviewStats(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get communications overview stats", err.Error())
		return
	}

	JSON(w, http.StatusOK, stats)
}

// ListOutbox returns paginated outbox messages with optional state/channel filters.
func (h *NotificationHandler) ListOutbox(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := int32(1)
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}
	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = int32(l)
	}

	var statePtr *string
	if val := q.Get("delivery_state"); val != "" {
		statePtr = &val
	}
	var channelPtr *string
	if val := q.Get("channel"); val != "" {
		channelPtr = &val
	}

	resp, err := h.notifService.ListOutboxAdmin(r.Context(), statePtr, channelPtr, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list outbox queue", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// ListFailed returns failed and dead-lettered outbox items.
func (h *NotificationHandler) ListFailed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := int32(1)
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}
	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = int32(l)
	}

	resp, err := h.notifService.ListFailedOutboxAdmin(r.Context(), page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list failed outbox messages", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// RetryDelivery resets a failed outbox message to pending state for immediate re-attempt.
func (h *NotificationHandler) RetryDelivery(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Outbox delivery ID is required", nil)
		return
	}

	item, err := h.notifService.RetryOutboxItem(r.Context(), user.ID, id)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Outbox item not found", nil)
		return
	}

	JSON(w, http.StatusOK, item)
}

// ListDeliveries returns audit records of provider delivery attempts.
func (h *NotificationHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := int32(1)
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}
	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = int32(l)
	}

	var provPtr *string
	if val := q.Get("provider"); val != "" {
		provPtr = &val
	}
	var chanPtr *string
	if val := q.Get("channel"); val != "" {
		chanPtr = &val
	}
	var statPtr *string
	if val := q.Get("status"); val != "" {
		statPtr = &val
	}

	resp, err := h.notifService.ListDeliveriesAdmin(r.Context(), provPtr, chanPtr, statPtr, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list delivery history", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// ListTemplates returns notification templates for communications management.
func (h *NotificationHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.notifService.ListTemplatesAdmin(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list templates", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"templates": templates,
	})
}

type updateTemplateReq struct {
	TitleTemplate string `json:"title_template"`
	BodyTemplate  string `json:"body_template"`
	IsActive      bool   `json:"is_active"`
}

// UpdateTemplate updates a template's content and version.
func (h *NotificationHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Template ID is required", nil)
		return
	}

	var req updateTemplateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
		return
	}

	tpl, err := h.notifService.UpdateTemplateAdmin(r.Context(), user.ID, id, req.TitleTemplate, req.BodyTemplate, req.IsActive)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update template", err.Error())
		return
	}

	JSON(w, http.StatusOK, tpl)
}

// ListSchedules returns automated cleanup reminder schedules.
func (h *NotificationHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := int32(1)
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = int32(p)
	}
	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = int32(l)
	}

	var statPtr *string
	if val := q.Get("status"); val != "" {
		statPtr = &val
	}

	resp, err := h.notifService.ListSchedulesAdmin(r.Context(), statPtr, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list schedules", err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// GetProviderHealth returns live provider configuration and outbox worker health.
func (h *NotificationHandler) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.notifService.GetProviderHealth(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get provider health", err.Error())
		return
	}

	JSON(w, http.StatusOK, health)
}
