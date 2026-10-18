package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

type HeroMediaHandler struct {
	heroMediaService *service.HeroMediaService
}

func NewHeroMediaHandler(heroMediaService *service.HeroMediaService) *HeroMediaHandler {
	return &HeroMediaHandler{
		heroMediaService: heroMediaService,
	}
}

// GetActiveHeroMedia handles the public homepage request for the active hero background video.
func (h *HeroMediaHandler) GetActiveHeroMedia(w http.ResponseWriter, r *http.Request) {
	resp, err := h.heroMediaService.GetActiveHeroMedia(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, resp)
}

// ListHeroMedia returns all hero media items for the administrative console.
func (h *HeroMediaHandler) ListHeroMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	items, err := h.heroMediaService.ListHeroMedia(r.Context(), user)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	if items == nil {
		items = []*service.HeroMediaDetailDTO{}
	}
	JSON(w, http.StatusOK, items)
}

// AuthorizeHeroMediaUpload generates a presigned R2 upload URL for direct client-to-R2 upload.
func (h *HeroMediaHandler) AuthorizeHeroMediaUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req struct {
		Filename      string `json:"filename"`
		ContentType   string `json:"content_type"`
		FileSizeBytes int64  `json:"file_size_bytes"`
		Title         string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	res, err := h.heroMediaService.AuthorizeHeroVideoUpload(r.Context(), user, req.Filename, req.ContentType, req.FileSizeBytes, req.Title)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Video storage service is unconfigured in this environment.", nil)
			return
		}
		if errors.Is(err, storage.ErrVideoEmpty) || errors.Is(err, storage.ErrVideoTooLarge) || errors.Is(err, storage.ErrInvalidVideoType) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_VIDEO", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "UPLOAD_AUTHORIZATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// UploadHeroMediaDirect handles multipart direct upload fallback.
func (h *HeroMediaHandler) UploadHeroMediaDirect(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	// Limit body to 55 MB (50MB video + multipart overhead)
	r.Body = http.MaxBytesReader(w, r.Body, 55*1024*1024)
	if err := r.ParseMultipartForm(50 * 1024 * 1024); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "Video upload exceeds maximum allowed limit of 50 MB", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("video")
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "MISSING_FILE", "A multipart file field named 'file' or 'video' is required", nil)
			return
		}
	}
	defer file.Close()

	title := r.FormValue("title")

	res, err := h.heroMediaService.UploadHeroVideoDirect(r.Context(), user, header.Filename, title, file, header.Size)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Video storage service is unconfigured in this environment.", nil)
			return
		}
		if errors.Is(err, storage.ErrVideoEmpty) || errors.Is(err, storage.ErrVideoTooLarge) ||
			errors.Is(err, storage.ErrInvalidVideoType) || errors.Is(err, storage.ErrCorruptVideo) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_VIDEO", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, res)
}

// PublishHeroMedia publishes a video as the active hero background video.
func (h *HeroMediaHandler) PublishHeroMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = req.ID
		}
	}

	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Hero media ID is required", nil)
		return
	}

	res, err := h.heroMediaService.PublishHeroMedia(r.Context(), user, id)
	if err != nil {
		if errors.Is(err, service.ErrHeroMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Hero media record not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "PUBLISH_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// ToggleHeroMedia enables or disables hero video playback.
func (h *HeroMediaHandler) ToggleHeroMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload: 'enabled' boolean required", nil)
		return
	}

	res, err := h.heroMediaService.ToggleHeroMedia(r.Context(), user, req.Enabled)
	if err != nil {
		if errors.Is(err, service.ErrNoPublishedHeroMedia) {
			ErrorJSON(w, http.StatusBadRequest, "NO_PUBLISHED_MEDIA", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "TOGGLE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// UploadHeroPoster uploads an optional poster image for a hero video.
func (h *HeroMediaHandler) UploadHeroPoster(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Hero media ID is required", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "Poster upload exceeds maximum allowed limit of 5 MB", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("poster")
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "MISSING_FILE", "A multipart file field named 'file' or 'poster' is required", nil)
			return
		}
	}
	defer file.Close()

	res, err := h.heroMediaService.UploadHeroPoster(r.Context(), user, id, header.Filename, file, header.Size)
	if err != nil {
		if errors.Is(err, service.ErrHeroMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Hero media record not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrImageEmpty) || errors.Is(err, storage.ErrImageTooLarge) ||
			errors.Is(err, storage.ErrInvalidImageType) || errors.Is(err, storage.ErrCorruptImage) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_IMAGE", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "POSTER_UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// DeleteHeroMedia removes a hero media item and cleans up storage.
func (h *HeroMediaHandler) DeleteHeroMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Hero media ID is required", nil)
		return
	}

	err := h.heroMediaService.DeleteHeroMedia(r.Context(), user, id)
	if err != nil {
		if errors.Is(err, service.ErrHeroMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Hero media record not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedMediaAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status":  "deleted",
		"message": "Homepage hero media removed successfully.",
	})
}
