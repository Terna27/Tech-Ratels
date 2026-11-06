package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

func (h *MemberHandler) GetMyMedia(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, 401, "UNAUTHORIZED", "Sign in to view your profile images", nil)
		return
	}
	result, err := h.authService.MemberMediaDetails(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, 400, "MEDIA_UNAVAILABLE", "Unable to read profile image settings", nil)
		return
	}
	JSON(w, 200, result)
}

func (h *MemberHandler) UploadMyMedia(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, 401, "UNAUTHORIZED", "Sign in to upload profile images", nil)
		return
	}
	kind := chi.URLParam(r, "kind")
	if kind != "avatar" && kind != "cover" {
		ErrorJSON(w, 400, "INVALID_IMAGE_KIND", "Choose avatar or cover", nil)
		return
	}
	if h.mediaStorage == nil || !h.mediaStorage.IsConfigured() {
		ErrorJSON(w, 503, "STORAGE_UNAVAILABLE", "Image storage is not configured", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		ErrorJSON(w, 400, "UPLOAD_TOO_LARGE", "Image upload must be no larger than 5 MB", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("file")
	if err != nil {
		ErrorJSON(w, 400, "MISSING_IMAGE", "Select an image to upload", nil)
		return
	}
	defer file.Close()
	result, err := h.authService.UploadMemberMedia(r.Context(), user.ID, kind, file, h.mediaStorage)
	if errors.Is(err, service.ErrAccountInactive) {
		ErrorJSON(w, 403, "ACCOUNT_INACTIVE", "Inactive members cannot edit their images", nil)
		return
	}
	if errors.Is(err, storage.ErrStorageUnconfigured) {
		ErrorJSON(w, 503, "STORAGE_UNAVAILABLE", "Image storage is not configured", nil)
		return
	}
	if err != nil {
		ErrorJSON(w, 400, "INVALID_IMAGE", "Image must be JPEG, PNG or WebP, within 5 MB, with adequate dimensions", nil)
		return
	}
	JSON(w, 200, result)
}

func (h *MemberHandler) PositionMyMedia(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, 401, "UNAUTHORIZED", "Sign in to adjust your images", nil)
		return
	}
	kind := chi.URLParam(r, "kind")
	var req struct {
		X    int `json:"x"`
		Y    int `json:"y"`
		Zoom int `json:"zoom"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ErrorJSON(w, 400, "INVALID_REQUEST", "Invalid position data", nil)
		return
	}
	if err := h.authService.UpdateMemberMediaPosition(r.Context(), user.ID, kind, req.X, req.Y, req.Zoom); err != nil {
		if errors.Is(err, service.ErrAccountInactive) {
			ErrorJSON(w, 403, "ACCOUNT_INACTIVE", "Account inactive", nil)
			return
		}
		ErrorJSON(w, 400, "INVALID_POSITION", "Position must be between 0 and 100, zoom between 100 and 200", nil)
		return
	}
	result, err := h.authService.MemberMediaDetails(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, 500, "MEDIA_UNAVAILABLE", "Image settings unavailable", nil)
		return
	}
	JSON(w, 200, result)
}

func (h *MemberHandler) DeleteMyMedia(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, 401, "UNAUTHORIZED", "Sign in to remove your images", nil)
		return
	}
	if err := h.authService.RemoveMemberMedia(r.Context(), user.ID, chi.URLParam(r, "kind"), h.mediaStorage); err != nil {
		if errors.Is(err, service.ErrAccountInactive) {
			ErrorJSON(w, 403, "ACCOUNT_INACTIVE", "Account inactive", nil)
			return
		}
		ErrorJSON(w, 400, "REMOVE_FAILED", "Could not remove image", nil)
		return
	}
	JSON(w, 200, map[string]string{"status": "removed"})
}

func (h *MemberHandler) GetMyCoverImage(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, 401, "UNAUTHORIZED", "Sign in to view cover image", nil)
		return
	}
	data, err := h.authService.MemberCoverImage(r.Context(), user.ID, h.mediaStorage)
	if err != nil {
		ErrorJSON(w, 404, "COVER_NOT_FOUND", "Cover photo unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
