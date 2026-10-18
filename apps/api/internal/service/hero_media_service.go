package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/storage"
)

var (
	ErrUnauthorizedMediaAction = errors.New("only authorized administrators may manage homepage hero media")
	ErrHeroMediaNotFound       = errors.New("homepage hero media record not found")
	ErrNoPublishedHeroMedia    = errors.New("no previously published hero video found to re-enable; please upload or publish a video first")
)

type HeroMediaService struct {
	repo    *repository.Repository
	audit   *AuditService
	storage storage.StorageService
}

func NewHeroMediaService(repo *repository.Repository, audit *AuditService, storage storage.StorageService) *HeroMediaService {
	return &HeroMediaService{
		repo:    repo,
		audit:   audit,
		storage: storage,
	}
}

// ActiveHeroMediaResponse is returned by the public homepage endpoint.
type ActiveHeroMediaResponse struct {
	Active bool                `json:"active"`
	Media  *HeroMediaDetailDTO `json:"media"`
}

// HeroMediaDetailDTO describes a homepage hero media object.
type HeroMediaDetailDTO struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	VideoKey          string     `json:"video_key"`
	VideoURL          string     `json:"video_url"`
	PosterKey         string     `json:"poster_key,omitempty"`
	PosterURL         string     `json:"poster_url,omitempty"`
	ContentType       string     `json:"content_type"`
	FileSizeBytes     int64      `json:"file_size_bytes"`
	DurationSeconds   float64    `json:"duration_seconds,omitempty"`
	IsActive          bool       `json:"is_active"`
	CreatedByUserID   string     `json:"created_by_user_id"`
	PublishedByUserID string     `json:"published_by_user_id,omitempty"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// PresignedHeroUploadResult returns presigned upload credentials for direct-to-R2 upload.
type PresignedHeroUploadResult struct {
	ID               string `json:"id"`
	UploadURL        string `json:"upload_url"`
	ObjectKey        string `json:"object_key"`
	PublicURL        string `json:"public_url"`
	ContentType      string `json:"content_type"`
	FileSizeBytes    int64  `json:"file_size_bytes"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// ToggleHeroMediaResult is returned when playback is enabled/disabled.
type ToggleHeroMediaResult struct {
	Active  bool                `json:"active"`
	Message string              `json:"message"`
	Media   *HeroMediaDetailDTO `json:"media,omitempty"`
}

func isHeroAdmin(user *db.User) bool {
	if user == nil {
		return false
	}
	role := user.PrimaryRole
	return role == "super_admin" || role == "national_admin"
}

// GetActiveHeroMedia returns the active hero video configuration for the public homepage.
func (s *HeroMediaService) GetActiveHeroMedia(ctx context.Context) (*ActiveHeroMediaResponse, error) {
	row, err := s.repo.Queries().GetActiveHeroMedia(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ActiveHeroMediaResponse{
				Active: false,
				Media:  nil,
			}, nil
		}
		return nil, fmt.Errorf("failed to fetch active hero media: %w", err)
	}

	dto := toHeroMediaDTO(row)
	return &ActiveHeroMediaResponse{
		Active: true,
		Media:  dto,
	}, nil
}

// ListHeroMedia returns all uploaded hero media for admin oversight.
func (s *HeroMediaService) ListHeroMedia(ctx context.Context, adminUser *db.User) ([]*HeroMediaDetailDTO, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	rows, err := s.repo.Queries().ListHeroMedia(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list hero media records: %w", err)
	}

	dtos := make([]*HeroMediaDetailDTO, len(rows))
	for i, r := range rows {
		dtos[i] = toHeroMediaDTO(r)
	}
	return dtos, nil
}

// AuthorizeHeroVideoUpload validates upload metadata and generates a presigned R2 upload URL.
func (s *HeroMediaService) AuthorizeHeroVideoUpload(ctx context.Context, adminUser *db.User, filename, contentType string, fileSize int64, title string) (*PresignedHeroUploadResult, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	canonicalExt, cleanContentType, err := storage.ValidateHeroVideoUploadRequest(filename, contentType, fileSize)
	if err != nil {
		return nil, err
	}

	if !s.storage.IsConfigured() {
		return nil, storage.ErrStorageUnconfigured
	}

	objectKey := storage.GenerateHeroVideoKey(canonicalExt)
	expiry := 15 * time.Minute
	uploadURL, err := s.storage.GeneratePresignedUpload(ctx, objectKey, cleanContentType, fileSize, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate R2 presigned video upload URL: %w", err)
	}

	publicURL := s.storage.GetPublicURL(objectKey)
	recordID := uuid.New().String()

	cleanTitle := title
	if cleanTitle == "" {
		cleanTitle = "Homepage Hero Background Video"
	}

	// Persist pending record in DB (inactive until explicitly published)
	_, err = s.repo.Queries().CreateHeroMedia(ctx, db.CreateHeroMediaParams{
		ID:                recordID,
		Title:             cleanTitle,
		VideoKey:          objectKey,
		VideoUrl:          publicURL,
		PosterKey:         pgtype.Text{Valid: false},
		PosterUrl:         pgtype.Text{Valid: false},
		ContentType:       cleanContentType,
		FileSizeBytes:     fileSize,
		DurationSeconds:   pgtype.Numeric{Valid: false},
		IsActive:          false,
		CreatedByUserID:   adminUser.ID,
		PublishedByUserID: pgtype.Text{Valid: false},
		PublishedAt:       pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hero media record: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "authorize_hero_video_upload", "homepage_hero_media", recordID, nil, nil, nil, nil)

	return &PresignedHeroUploadResult{
		ID:               recordID,
		UploadURL:        uploadURL,
		ObjectKey:        objectKey,
		PublicURL:        publicURL,
		ContentType:      cleanContentType,
		FileSizeBytes:    fileSize,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

// UploadHeroVideoDirect handles fallback multipart uploads directly to R2 through the backend.
func (s *HeroMediaService) UploadHeroVideoDirect(ctx context.Context, adminUser *db.User, filename, title string, r io.Reader, size int64) (*HeroMediaDetailDTO, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	if !s.storage.IsConfigured() {
		return nil, storage.ErrStorageUnconfigured
	}

	videoBytes, meta, err := storage.InspectAndValidateVideo(r, size)
	if err != nil {
		return nil, err
	}

	objectKey := storage.GenerateHeroVideoKey(meta.Format)
	publicURL, err := s.storage.UploadObject(ctx, objectKey, meta.ContentType, bytes.NewReader(videoBytes), meta.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to upload hero video to R2: %w", err)
	}

	recordID := uuid.New().String()
	cleanTitle := title
	if cleanTitle == "" {
		cleanTitle = "Homepage Hero Background Video"
	}

	created, err := s.repo.Queries().CreateHeroMedia(ctx, db.CreateHeroMediaParams{
		ID:                recordID,
		Title:             cleanTitle,
		VideoKey:          objectKey,
		VideoUrl:          publicURL,
		PosterKey:         pgtype.Text{Valid: false},
		PosterUrl:         pgtype.Text{Valid: false},
		ContentType:       meta.ContentType,
		FileSizeBytes:     meta.Size,
		DurationSeconds:   pgtype.Numeric{Valid: false},
		IsActive:          false,
		CreatedByUserID:   adminUser.ID,
		PublishedByUserID: pgtype.Text{Valid: false},
		PublishedAt:       pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save hero media record: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "upload_hero_video_direct", "homepage_hero_media", recordID, nil, nil, nil, nil)

	return toHeroMediaDTO(created), nil
}

// PublishHeroMedia atomically publishes the specified hero media as the active homepage hero video.
func (s *HeroMediaService) PublishHeroMedia(ctx context.Context, adminUser *db.User, id string) (*HeroMediaDetailDTO, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	// Verify target media exists
	_, err := s.repo.Queries().GetHeroMediaByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHeroMediaNotFound
		}
		return nil, fmt.Errorf("failed to find hero media: %w", err)
	}

	var published db.HomepageHeroMedium
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Deactivate any currently active video atomically
		if err := q.DeactivateAllHeroMedia(ctx); err != nil {
			return err
		}

		// Activate the target video
		res, err := q.PublishHeroMedia(ctx, db.PublishHeroMediaParams{
			ID:                id,
			PublishedByUserID: pgtype.Text{String: adminUser.ID, Valid: true},
		})
		if err != nil {
			return err
		}
		published = res
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to publish hero media: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "publish_homepage_hero_media", "homepage_hero_media", id, nil, nil, nil, nil)

	return toHeroMediaDTO(published), nil
}

// ToggleHeroMedia toggles video playback on or off without deleting the media record.
func (s *HeroMediaService) ToggleHeroMedia(ctx context.Context, adminUser *db.User, enabled bool) (*ToggleHeroMediaResult, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	if !enabled {
		// Disable playback -> restore default green gradient
		if err := s.repo.Queries().DeactivateAllHeroMedia(ctx); err != nil {
			return nil, fmt.Errorf("failed to deactivate hero media: %w", err)
		}

		_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "disable_homepage_hero_media", "homepage_hero_media", "all", nil, nil, nil, nil)

		return &ToggleHeroMediaResult{
			Active:  false,
			Message: "Hero video playback disabled; default green gradient background restored.",
			Media:   nil,
		}, nil
	}

	// Enable playback -> re-activate the latest published video
	latest, err := s.repo.Queries().GetLatestPublishedHeroMedia(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoPublishedHeroMedia
		}
		return nil, fmt.Errorf("failed to query latest published video: %w", err)
	}

	var activated db.HomepageHeroMedium
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		if err := q.DeactivateAllHeroMedia(ctx); err != nil {
			return err
		}
		res, err := q.PublishHeroMedia(ctx, db.PublishHeroMediaParams{
			ID:                latest.ID,
			PublishedByUserID: pgtype.Text{String: adminUser.ID, Valid: true},
		})
		if err != nil {
			return err
		}
		activated = res
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enable hero media: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "enable_homepage_hero_media", "homepage_hero_media", latest.ID, nil, nil, nil, nil)

	return &ToggleHeroMediaResult{
		Active:  true,
		Message: "Hero video playback enabled.",
		Media:   toHeroMediaDTO(activated),
	}, nil
}

// UploadHeroPoster allows uploading an optional poster image for a hero video.
func (s *HeroMediaService) UploadHeroPoster(ctx context.Context, adminUser *db.User, id string, filename string, r io.Reader, size int64) (*HeroMediaDetailDTO, error) {
	if !isHeroAdmin(adminUser) {
		return nil, ErrUnauthorizedMediaAction
	}

	media, err := s.repo.Queries().GetHeroMediaByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHeroMediaNotFound
		}
		return nil, err
	}

	imgBytes, meta, err := storage.InspectAndValidateImage(r, size)
	if err != nil {
		return nil, err
	}

	posterKey := storage.GenerateHeroPosterKey(media.VideoKey, meta.Format)
	posterURL, err := s.storage.UploadObject(ctx, posterKey, meta.ContentType, bytes.NewReader(imgBytes), meta.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to upload poster image to R2: %w", err)
	}

	updated, err := s.repo.Queries().UpdateHeroMediaPoster(ctx, db.UpdateHeroMediaPosterParams{
		ID:        id,
		PosterKey: pgtype.Text{String: posterKey, Valid: true},
		PosterUrl: pgtype.Text{String: posterURL, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update hero media poster: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "upload_hero_poster", "homepage_hero_media", id, nil, nil, nil, nil)

	return toHeroMediaDTO(updated), nil
}

// DeleteHeroMedia removes a video record and cleans up the remote R2 assets.
func (s *HeroMediaService) DeleteHeroMedia(ctx context.Context, adminUser *db.User, id string) error {
	if !isHeroAdmin(adminUser) {
		return ErrUnauthorizedMediaAction
	}

	media, err := s.repo.Queries().GetHeroMediaByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrHeroMediaNotFound
		}
		return fmt.Errorf("failed to find hero media: %w", err)
	}

	// Delete from storage (best effort; errors logged but don't block DB cleanup)
	if media.VideoKey != "" {
		_ = s.storage.DeleteObject(ctx, media.VideoKey)
	}
	if media.PosterKey.Valid && media.PosterKey.String != "" {
		_ = s.storage.DeleteObject(ctx, media.PosterKey.String)
	}

	if err := s.repo.Queries().DeleteHeroMedia(ctx, id); err != nil {
		return fmt.Errorf("failed to delete hero media database record: %w", err)
	}

	_ = s.audit.Log(ctx, &adminUser.ID, &adminUser.PrimaryRole, "delete_homepage_hero_media", "homepage_hero_media", id, nil, nil, nil, nil)

	return nil
}

func toHeroMediaDTO(row db.HomepageHeroMedium) *HeroMediaDetailDTO {
	dto := &HeroMediaDetailDTO{
		ID:              row.ID,
		Title:           row.Title,
		VideoKey:        row.VideoKey,
		VideoURL:        row.VideoUrl,
		ContentType:     row.ContentType,
		FileSizeBytes:   row.FileSizeBytes,
		IsActive:        row.IsActive,
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}

	if row.PosterKey.Valid {
		dto.PosterKey = row.PosterKey.String
	}
	if row.PosterUrl.Valid {
		dto.PosterURL = row.PosterUrl.String
	}
	if row.PublishedByUserID.Valid {
		dto.PublishedByUserID = row.PublishedByUserID.String
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		dto.PublishedAt = &t
	}
	if row.DurationSeconds.Valid {
		f, _ := row.DurationSeconds.Float64Value()
		dto.DurationSeconds = f.Float64
	}

	return dto
}
