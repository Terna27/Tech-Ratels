package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var (
	ErrMediaNotFound           = errors.New("event media record not found")
	ErrEventNotEligible        = errors.New("media can only be submitted for published or active cleanup events")
	ErrConsentRequired         = errors.New("mandatory participant and privacy consent declaration is required")
	ErrGuardianConsentRequired = errors.New("guardian consent declaration is mandatory when minors are pictured")
	ErrUnauthorizedReviewer    = errors.New("insufficient permissions or geographic jurisdiction to moderate this media")
	ErrNotMediaUploader        = errors.New("only the original uploader may withdraw or request takedown for this media submission")
	ErrReportNotFound          = errors.New("media report record not found")
	ErrTakedownNotFound        = errors.New("media takedown request record not found")
	ErrMissingReason           = errors.New("a clear and documented reason must be provided")
	ErrMediaAlreadyApproved    = errors.New("media is already approved and published")
)

type EventMedia = db.EventMedium

type GalleryService struct {
	repo    *repository.Repository
	storage storage.StorageService
	audit   *AuditService
}

func NewGalleryService(repo *repository.Repository, store storage.StorageService, audit *AuditService) *GalleryService {
	return &GalleryService{
		repo:    repo,
		storage: store,
		audit:   audit,
	}
}

// -------------------------------------------------------------
// DTOs
// -------------------------------------------------------------

type AuthorizeGalleryUploadRequest struct {
	EventID       string `json:"event_id"`
	MediaType     string `json:"media_type"` // 'image', 'video'
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	FileSizeBytes int64  `json:"file_size_bytes"`
}

type AuthorizeGalleryUploadResult struct {
	UploadURL        string `json:"upload_url"`
	StorageKey       string `json:"storage_key"`
	MediaType        string `json:"media_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

type GalleryConsentInput struct {
	HasOwnerPermission        bool    `json:"has_owner_permission"`
	UnderstandsPublicDisplay  bool    `json:"understands_public_display"`
	HasIdentifiableConsent    bool    `json:"has_identifiable_consent"`
	HasNoConfidentialInfo     bool    `json:"has_no_confidential_info"`
	HasMinors                 bool    `json:"has_minors"`
	HasGuardianConsent        bool    `json:"has_guardian_consent"`
	ConsentNotes              *string `json:"consent_notes"`
}

type SubmitGalleryMediaRequest struct {
	EventID                 string              `json:"event_id"`
	MediaType               string              `json:"media_type"`
	Caption                 string              `json:"caption"`
	Description             *string             `json:"description"`
	StorageKey              string              `json:"storage_key"`
	MimeType                string              `json:"mime_type"`
	FileSizeBytes           int64               `json:"file_size_bytes"`
	Width                   *int32              `json:"width"`
	Height                  *int32              `json:"height"`
	DurationSeconds         *float64            `json:"duration_seconds"`
	AllowPublicAttribution  bool                `json:"allow_public_attribution"`
	Consent                 GalleryConsentInput `json:"consent"`
}

type PublicGalleryFilter struct {
	MediaType *string `json:"media_type"`
	ZoneID    *string `json:"zone_id"`
	StateID   *string `json:"state_id"`
	LgaID     *string `json:"lga_id"`
	EventID   *string `json:"event_id"`
	Year      *int32  `json:"year"`
	Page      int32   `json:"page"`
	Limit     int32   `json:"limit"`
}

type AdminGalleryFilter struct {
	Status  *string `json:"status"`
	ZoneID  *string `json:"zone_id"`
	StateID *string `json:"state_id"`
	LgaID   *string `json:"lga_id"`
	EventID *string `json:"event_id"`
	Page    int32   `json:"page"`
	Limit   int32   `json:"limit"`
}

type PaginatedPublicGalleryResponse struct {
	Items []db.ListPublicGalleryMediaRow `json:"items"`
	Total int64                          `json:"total"`
	Page  int32                          `json:"page"`
	Limit int32                          `json:"limit"`
}

type PaginatedAdminGalleryResponse struct {
	Items []db.ListAdminEventMediaRow `json:"items"`
	Total int64                       `json:"total"`
	Page  int32                       `json:"page"`
	Limit int32                       `json:"limit"`
}

type PaginatedMyMediaResponse struct {
	Items []db.ListMyEventMediaRow `json:"items"`
	Total int64                    `json:"total"`
	Page  int32                    `json:"page"`
	Limit int32                    `json:"limit"`
}

// -------------------------------------------------------------
// Member Upload Workflows
// -------------------------------------------------------------

// AuthorizeUpload validates event and generates a presigned quarantine upload key.
func (s *GalleryService) AuthorizeUpload(ctx context.Context, user *db.User, req AuthorizeGalleryUploadRequest) (*AuthorizeGalleryUploadResult, error) {
	// 1. Verify Event
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, req.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	if event.Status == "draft" || event.Status == "cancelled" {
		return nil, ErrEventNotEligible
	}

	// 2. Validate Media Constraints
	canonicalExt, cleanMime, err := storage.ValidateGalleryUploadRequest(req.Filename, req.ContentType, req.FileSizeBytes, req.MediaType)
	if err != nil {
		return nil, err
	}

	// 3. Generate Quarantine Key
	storageKey := storage.GenerateGalleryPendingKey(req.MediaType, canonicalExt)

	expiry := 20 * time.Minute
	uploadURL, err := s.storage.GeneratePresignedUpload(ctx, storageKey, cleanMime, req.FileSizeBytes, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload authorization: %w", err)
	}

	return &AuthorizeGalleryUploadResult{
		UploadURL:        uploadURL,
		StorageKey:       storageKey,
		MediaType:        req.MediaType,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

// SubmitMedia records an uploaded media item into the pending moderation queue.
func (s *GalleryService) SubmitMedia(ctx context.Context, user *db.User, req SubmitGalleryMediaRequest) (*db.EventMedium, error) {
	// 1. Validate Consent Checkboxes
	if !req.Consent.HasOwnerPermission || !req.Consent.UnderstandsPublicDisplay ||
		!req.Consent.HasIdentifiableConsent || !req.Consent.HasNoConfidentialInfo {
		return nil, ErrConsentRequired
	}
	if req.Consent.HasMinors && !req.Consent.HasGuardianConsent {
		return nil, ErrGuardianConsentRequired
	}

	if strings.TrimSpace(req.Caption) == "" {
		return nil, errors.New("caption is required")
	}

	// 2. Verify Event and retrieve geographic attribution
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, req.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	if event.Status == "draft" || event.Status == "cancelled" {
		return nil, ErrEventNotEligible
	}

	mediaID := security.GenerateUUID()
	consentID := security.GenerateUUID()

	var createdMedia db.EventMedium
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		var wVal, hVal pgtype.Int4
		if req.Width != nil && *req.Width > 0 {
			wVal = pgtype.Int4{Int32: *req.Width, Valid: true}
		}
		if req.Height != nil && *req.Height > 0 {
			hVal = pgtype.Int4{Int32: *req.Height, Valid: true}
		}

		var durVal pgtype.Numeric
		if req.DurationSeconds != nil && *req.DurationSeconds > 0 {
			_ = durVal.Scan(fmt.Sprintf("%.2f", *req.DurationSeconds))
		}

		m, err := q.CreateEventMedia(ctx, db.CreateEventMediaParams{
			ID:                     mediaID,
			EventID:                event.ID,
			UploadedByUserID:       user.ID,
			MediaType:              req.MediaType,
			Caption:                strings.TrimSpace(req.Caption),
			Description:            Text(req.Description),
			StorageKey:             req.StorageKey,
			PublicUrl:              pgtype.Text{Valid: false},    // Private while pending
			ThumbnailUrl:           pgtype.Text{Valid: false}, // Private while pending
			MimeType:               req.MimeType,
			FileSizeBytes:          req.FileSizeBytes,
			Width:                  wVal,
			Height:                 hVal,
			DurationSeconds:        durVal,
			ModerationStatus:       "pending_review",
			RejectionReason:        pgtype.Text{Valid: false},
			ReviewedByUserID:       pgtype.Text{Valid: false},
			ReviewedAt:             pgtype.Timestamptz{Valid: false},
			IsPublished:            false,
			PublishedAt:            pgtype.Timestamptz{Valid: false},
			IsHidden:               false,
			IsFeatured:             false,
			MetadataSanitized:      false,
			SanitizedAt:            pgtype.Timestamptz{Valid: false},
			AllowPublicAttribution: req.AllowPublicAttribution,
			StateID:                event.StateID,
			LgaID:                  event.LgaID,
		})
		if err != nil {
			return fmt.Errorf("failed to create event media record: %w", err)
		}
		createdMedia = m

		_, err = q.CreateEventMediaConsent(ctx, db.CreateEventMediaConsentParams{
			ID:                       consentID,
			MediaID:                  mediaID,
			HasOwnerPermission:       req.Consent.HasOwnerPermission,
			UnderstandsPublicDisplay: req.Consent.UnderstandsPublicDisplay,
			HasIdentifiableConsent:   req.Consent.HasIdentifiableConsent,
			HasNoConfidentialInfo:    req.Consent.HasNoConfidentialInfo,
			HasMinors:                req.Consent.HasMinors,
			HasGuardianConsent:       req.Consent.HasGuardianConsent,
			ConsentNotes:             Text(req.Consent.ConsentNotes),
			DeclaredAt:               TimestamptzFromTime(time.Now()),
		})
		if err != nil {
			return fmt.Errorf("failed to record media consent: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "submit_event_media", "event_media", mediaID, &scopeEvent, &event.ID, nil, nil)

	return &createdMedia, nil
}

// SubmitDirectMedia handles direct file upload (fallback for client-side presign issues).
func (s *GalleryService) SubmitDirectMedia(ctx context.Context, user *db.User, eventID, mediaType, caption string, description *string, allowAttribution bool, consent GalleryConsentInput, r io.Reader, filename, contentType string, size int64) (*db.EventMedium, error) {
	// 1. Inspect and validate file bytes
	data, mime, width, height, err := storage.InspectAndValidateGalleryUpload(r, size, mediaType)
	if err != nil {
		return nil, err
	}

	canonicalExt, _, err := storage.ValidateGalleryUploadRequest(filename, contentType, int64(len(data)), mediaType)
	if err != nil {
		return nil, err
	}

	// 2. Upload to private quarantine prefix
	quarantineKey := storage.GenerateGalleryPendingKey(mediaType, canonicalExt)
	_, err = s.storage.UploadObject(ctx, quarantineKey, mime, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to upload media to storage: %w", err)
	}

	var wPtr, hPtr *int32
	if width > 0 {
		w32 := int32(width)
		wPtr = &w32
	}
	if height > 0 {
		h32 := int32(height)
		hPtr = &h32
	}

	return s.SubmitMedia(ctx, user, SubmitGalleryMediaRequest{
		EventID:                eventID,
		MediaType:              mediaType,
		Caption:                caption,
		Description:            description,
		StorageKey:             quarantineKey,
		MimeType:               mime,
		FileSizeBytes:          int64(len(data)),
		Width:                  wPtr,
		Height:                 hPtr,
		AllowPublicAttribution: allowAttribution,
		Consent:                consent,
	})
}

// ListMySubmissions returns media submitted by the authenticated user.
func (s *GalleryService) ListMySubmissions(ctx context.Context, user *db.User, page, limit int32) (*PaginatedMyMediaResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	items, err := s.repo.Queries().ListMyEventMedia(ctx, db.ListMyEventMediaParams{
		UploadedByUserID: user.ID,
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		return nil, err
	}

	total, _ := s.repo.Queries().CountMyEventMedia(ctx, user.ID)

	if items == nil {
		items = []db.ListMyEventMediaRow{}
	}

	return &PaginatedMyMediaResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// WithdrawSubmission allows an uploader to withdraw their pending or revision-requested submission.
func (s *GalleryService) WithdrawSubmission(ctx context.Context, user *db.User, mediaID, reason string) (*db.EventMedium, error) {
	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	if media.UploadedByUserID != user.ID {
		return nil, ErrNotMediaUploader
	}

	if media.ModerationStatus != "pending_review" && media.ModerationStatus != "revision_requested" {
		return nil, fmt.Errorf("%w: only pending or revision-requested media may be withdrawn", ErrInvalidStatusTransition)
	}

	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		cleanReason = "Withdrawn by uploader"
	}

	updated, err := s.repo.Queries().WithdrawEventMedia(ctx, db.WithdrawEventMediaParams{
		ID:               mediaID,
		RejectionReason:  TextFromString(cleanReason),
		UploadedByUserID: user.ID,
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "withdraw_event_media", "event_media", mediaID, nil, nil, &cleanReason, nil)

	return &updated, nil
}

// -------------------------------------------------------------
// Public Gallery Workflows
// -------------------------------------------------------------

func (s *GalleryService) ListPublicGallery(ctx context.Context, filter PublicGalleryFilter) (*PaginatedPublicGalleryResponse, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 24
	}
	offset := (page - 1) * limit

	items, err := s.repo.Queries().ListPublicGalleryMedia(ctx, db.ListPublicGalleryMediaParams{
		Limit:     limit,
		Offset:    offset,
		MediaType: Text(filter.MediaType),
		ZoneID:    Text(filter.ZoneID),
		StateID:   Text(filter.StateID),
		LgaID:     Text(filter.LgaID),
		EventID:   Text(filter.EventID),
		Year:      Int4(filter.Year),
	})
	if err != nil {
		return nil, err
	}

	total, _ := s.repo.Queries().CountPublicGalleryMedia(ctx, db.CountPublicGalleryMediaParams{
		MediaType: Text(filter.MediaType),
		ZoneID:    Text(filter.ZoneID),
		StateID:   Text(filter.StateID),
		LgaID:     Text(filter.LgaID),
		EventID:   Text(filter.EventID),
		Year:      Int4(filter.Year),
	})

	if items == nil {
		items = []db.ListPublicGalleryMediaRow{}
	}

	return &PaginatedPublicGalleryResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *GalleryService) GetPublicMediaDetail(ctx context.Context, mediaID string) (*db.GetEventMediaDetailForPublicRow, error) {
	media, err := s.repo.Queries().GetEventMediaDetailForPublic(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	return &media, nil
}

func (s *GalleryService) ListFeaturedGallery(ctx context.Context, limit int32) ([]db.ListFeaturedGalleryMediaRow, error) {
	if limit < 1 || limit > 20 {
		limit = 6
	}
	return s.repo.Queries().ListFeaturedGalleryMedia(ctx, limit)
}

func (s *GalleryService) ListEventGallery(ctx context.Context, eventID string, page, limit int32) ([]db.ListEventGalleryMediaRow, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}
	offset := (page - 1) * limit
	return s.repo.Queries().ListEventGalleryMedia(ctx, db.ListEventGalleryMediaParams{
		EventID: eventID,
		Limit:   limit,
		Offset:  offset,
	})
}

// -------------------------------------------------------------
// Public Reports & Takedowns
// -------------------------------------------------------------

type CreateMediaReportRequest struct {
	ReporterEmail *string `json:"reporter_email"`
	ReportReason  string  `json:"report_reason"` // 'privacy_violation', 'missing_consent', 'incorrect_event', 'inappropriate_content', etc.
	Details       string  `json:"details"`
}

func (s *GalleryService) ReportMedia(ctx context.Context, user *db.User, mediaID string, req CreateMediaReportRequest) (*db.MediaReport, error) {
	// Verify media exists
	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	cleanReason := strings.TrimSpace(req.ReportReason)
	if cleanReason == "" {
		return nil, errors.New("report reason is required")
	}
	cleanDetails := strings.TrimSpace(req.Details)
	if cleanDetails == "" {
		return nil, errors.New("report details are required")
	}

	var reporterID *string
	var reporterRole *string
	if user != nil {
		reporterID = &user.ID
		reporterRole = &user.PrimaryRole
	}

	reportID := security.GenerateUUID()
	report, err := s.repo.Queries().CreateMediaReport(ctx, db.CreateMediaReportParams{
		ID:             reportID,
		MediaID:        media.ID,
		ReporterUserID: Text(reporterID),
		ReporterEmail:  Text(req.ReporterEmail),
		ReportReason:   cleanReason,
		Details:        cleanDetails,
		Status:         "pending",
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, reporterID, reporterRole, "report_event_media", "event_media", mediaID, nil, nil, &cleanReason, nil)

	return &report, nil
}

type CreateMediaTakedownRequest struct {
	RequesterName        string `json:"requester_name"`
	RequesterEmail       string `json:"requester_email"`
	RelationshipToMedia  string `json:"relationship_to_media"`
	Reason               string `json:"reason"`
	Details              string `json:"details"`
}

func (s *GalleryService) SubmitTakedownRequest(ctx context.Context, user *db.User, mediaID string, req CreateMediaTakedownRequest) (*db.MediaTakedownRequest, error) {
	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	if strings.TrimSpace(req.RequesterName) == "" || strings.TrimSpace(req.RequesterEmail) == "" {
		return nil, errors.New("requester name and email are required")
	}
	if strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.Details) == "" {
		return nil, errors.New("reason and details are required")
	}

	var requesterID *string
	var role *string
	if user != nil {
		requesterID = &user.ID
		role = &user.PrimaryRole
	}

	takedownID := security.GenerateUUID()
	takedown, err := s.repo.Queries().CreateMediaTakedownRequest(ctx, db.CreateMediaTakedownRequestParams{
		ID:                  takedownID,
		MediaID:             media.ID,
		RequesterUserID:     Text(requesterID),
		RequesterName:       strings.TrimSpace(req.RequesterName),
		RequesterEmail:      strings.TrimSpace(req.RequesterEmail),
		RelationshipToMedia: strings.TrimSpace(req.RelationshipToMedia),
		Reason:              strings.TrimSpace(req.Reason),
		Details:             strings.TrimSpace(req.Details),
		Status:              "pending",
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, requesterID, role, "request_media_takedown", "event_media", mediaID, nil, nil, &req.Reason, nil)

	return &takedown, nil
}

// -------------------------------------------------------------
// Administrative Moderation Workflows
// -------------------------------------------------------------

func (s *GalleryService) ListAdminGallery(ctx context.Context, user *db.User, appointments []db.Appointment, filter AdminGalleryFilter) (*PaginatedAdminGalleryResponse, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 24
	}
	offset := (page - 1) * limit

	// Enforce geographic RBAC scoping on filter
	zoneID, stateID, lgaID := s.resolveAdminJurisdiction(user, appointments, filter.ZoneID, filter.StateID, filter.LgaID)

	items, err := s.repo.Queries().ListAdminEventMedia(ctx, db.ListAdminEventMediaParams{
		Limit:            limit,
		Offset:           offset,
		ModerationStatus: Text(filter.Status),
		ZoneID:           Text(zoneID),
		StateID:          Text(stateID),
		LgaID:            Text(lgaID),
		EventID:          Text(filter.EventID),
	})
	if err != nil {
		return nil, err
	}

	total, _ := s.repo.Queries().CountAdminEventMedia(ctx, db.CountAdminEventMediaParams{
		ModerationStatus: Text(filter.Status),
		ZoneID:           Text(zoneID),
		StateID:          Text(stateID),
		LgaID:            Text(lgaID),
		EventID:          Text(filter.EventID),
	})

	if items == nil {
		items = []db.ListAdminEventMediaRow{}
	}

	return &PaginatedAdminGalleryResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *GalleryService) GetAdminMediaDetail(ctx context.Context, user *db.User, appointments []db.Appointment, mediaID string) (*db.GetEventMediaDetailForAdminRow, *db.EventMediaConsent, []db.ListMediaModerationActionsByMediaIDRow, error) {
	media, err := s.repo.Queries().GetEventMediaDetailForAdmin(ctx, mediaID)
	if err != nil {
		return nil, nil, nil, ErrMediaNotFound
	}

	if !s.canModerateMedia(user, appointments, media.StateID, media.LgaID, media.ZoneID) {
		return nil, nil, nil, ErrUnauthorizedReviewer
	}

	consent, _ := s.repo.Queries().GetEventMediaConsentByMediaID(ctx, mediaID)
	actions, _ := s.repo.Queries().ListMediaModerationActionsByMediaID(ctx, mediaID)

	return &media, &consent, actions, nil
}

func (s *GalleryService) GetAdminOverview(ctx context.Context) (*db.GetAdminGalleryOverviewStatsRow, error) {
	stats, err := s.repo.Queries().GetAdminGalleryOverviewStats(ctx)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// ApproveMedia approves media, performs 100% EXIF/GPS metadata sanitization, creates thumbnail,
// publishes derivative to public R2 prefix, and sets public_url.
func (s *GalleryService) ApproveMedia(ctx context.Context, reviewer *db.User, appointments []db.Appointment, mediaID string, notes *string) (*db.EventMedium, error) {
	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	// 1. Anti-Self-Approval
	if media.UploadedByUserID == reviewer.ID {
		return nil, ErrSelfApprovalNotAllowed
	}

	// 2. Geographic RBAC check
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, media.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	state, err := s.repo.Queries().GetStateByID(ctx, event.StateID)
	if err != nil {
		return nil, err
	}
	if !s.canModerateMedia(reviewer, appointments, event.StateID, event.LgaID, state.ZoneID) {
		return nil, ErrUnauthorizedReviewer
	}

	if media.ModerationStatus == "approved" && media.IsPublished {
		return nil, ErrMediaAlreadyApproved
	}

	// 3. Metadata Sanitization & Derivative Generation
	var publicURL string
	var thumbURL string

	if media.MediaType == "image" {
		// Read raw quarantined original
		rawBytes, err := s.storage.ReadObject(ctx, media.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve unapproved original for sanitization: %w", err)
		}

		sanitizedBytes, thumbBytes, _, _, err := storage.SanitizeAndReencodeImage(rawBytes)
		if err != nil {
			return nil, fmt.Errorf("metadata sanitization failed: %w", err)
		}

		// Upload sanitized public derivative
		pubKey := storage.GenerateGalleryPublicKey(media.EventID, "image", "jpg")
		publicURL, err = s.storage.UploadObject(ctx, pubKey, "image/jpeg", bytes.NewReader(sanitizedBytes), int64(len(sanitizedBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to publish sanitized image: %w", err)
		}

		// Upload thumbnail
		thumbKey := storage.GenerateGalleryThumbnailKey(media.ID)
		thumbURL, err = s.storage.UploadObject(ctx, thumbKey, "image/jpeg", bytes.NewReader(thumbBytes), int64(len(thumbBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to publish thumbnail: %w", err)
		}

		// Purge quarantine original
		_ = s.storage.DeleteObject(ctx, media.StorageKey)
	} else if media.MediaType == "video" {
		// For videos, publish to public events video path
		rawBytes, err := s.storage.ReadObject(ctx, media.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve unapproved video for publishing: %w", err)
		}

		pubKey := storage.GenerateGalleryPublicKey(media.EventID, "video", "mp4")
		publicURL, err = s.storage.UploadObject(ctx, pubKey, media.MimeType, bytes.NewReader(rawBytes), int64(len(rawBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to publish video: %w", err)
		}
		thumbURL = publicURL

		// Purge quarantine original
		_ = s.storage.DeleteObject(ctx, media.StorageKey)
	}

	// 4. Update Database Record
	var updatedMedia db.EventMedium
	actionID := security.GenerateUUID()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		m, err := q.ApproveEventMedia(ctx, db.ApproveEventMediaParams{
			ID:               mediaID,
			PublicUrl:        TextFromString(publicURL),
			ThumbnailUrl:     TextFromString(thumbURL),
			ReviewedByUserID: TextFromString(reviewer.ID),
		})
		if err != nil {
			return err
		}
		updatedMedia = m

		cleanNotes := "Approved and published to public gallery after metadata sanitization"
		if notes != nil && strings.TrimSpace(*notes) != "" {
			cleanNotes = strings.TrimSpace(*notes)
		}

		_, err = q.CreateMediaModerationAction(ctx, db.CreateMediaModerationActionParams{
			ID:             actionID,
			MediaID:        mediaID,
			ReviewerUserID: reviewer.ID,
			Action:         "approve",
			Reason:         cleanNotes,
			Notes:          Text(notes),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "approve_event_media", "event_media", mediaID, &scopeEvent, &event.ID, notes, nil)

	return &updatedMedia, nil
}

// RejectMedia rejects a submission with a mandatory documented reason.
func (s *GalleryService) RejectMedia(ctx context.Context, reviewer *db.User, appointments []db.Appointment, mediaID, reason string) (*db.EventMedium, error) {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return nil, ErrMissingReason
	}

	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	if media.UploadedByUserID == reviewer.ID {
		return nil, ErrSelfApprovalNotAllowed
	}

	event, err := s.repo.Queries().GetEventInstanceByID(ctx, media.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	state, err := s.repo.Queries().GetStateByID(ctx, event.StateID)
	if err != nil {
		return nil, err
	}
	if !s.canModerateMedia(reviewer, appointments, event.StateID, event.LgaID, state.ZoneID) {
		return nil, ErrUnauthorizedReviewer
	}

	var updated db.EventMedium
	actionID := security.GenerateUUID()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		m, err := q.RejectEventMedia(ctx, db.RejectEventMediaParams{
			ID:               mediaID,
			RejectionReason:  TextFromString(cleanReason),
			ReviewedByUserID: TextFromString(reviewer.ID),
		})
		if err != nil {
			return err
		}
		updated = m

		_, err = q.CreateMediaModerationAction(ctx, db.CreateMediaModerationActionParams{
			ID:             actionID,
			MediaID:        mediaID,
			ReviewerUserID: reviewer.ID,
			Action:         "reject",
			Reason:         cleanReason,
			Notes:          TextFromString(cleanReason),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "reject_event_media", "event_media", mediaID, &scopeEvent, &event.ID, &cleanReason, nil)

	return &updated, nil
}

// RequestRevisionMedia requests the uploader to revise their submission (e.g. fix caption, provide clearer consent).
func (s *GalleryService) RequestRevisionMedia(ctx context.Context, reviewer *db.User, appointments []db.Appointment, mediaID, notes string) (*db.EventMedium, error) {
	cleanNotes := strings.TrimSpace(notes)
	if cleanNotes == "" {
		return nil, ErrMissingReason
	}

	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	if media.UploadedByUserID == reviewer.ID {
		return nil, ErrSelfApprovalNotAllowed
	}

	event, err := s.repo.Queries().GetEventInstanceByID(ctx, media.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	state, err := s.repo.Queries().GetStateByID(ctx, event.StateID)
	if err != nil {
		return nil, err
	}
	if !s.canModerateMedia(reviewer, appointments, event.StateID, event.LgaID, state.ZoneID) {
		return nil, ErrUnauthorizedReviewer
	}

	var updated db.EventMedium
	actionID := security.GenerateUUID()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		m, err := q.RequestRevisionEventMedia(ctx, db.RequestRevisionEventMediaParams{
			ID:               mediaID,
			RejectionReason:  TextFromString(cleanNotes),
			ReviewedByUserID: TextFromString(reviewer.ID),
		})
		if err != nil {
			return err
		}
		updated = m

		_, err = q.CreateMediaModerationAction(ctx, db.CreateMediaModerationActionParams{
			ID:             actionID,
			MediaID:        mediaID,
			ReviewerUserID: reviewer.ID,
			Action:         "request_revision",
			Reason:         cleanNotes,
			Notes:          TextFromString(cleanNotes),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "request_revision_event_media", "event_media", mediaID, &scopeEvent, &event.ID, &cleanNotes, nil)

	return &updated, nil
}

// RemoveMedia immediately unpublishes media, hides it from all queries, and deletes the public object from R2.
func (s *GalleryService) RemoveMedia(ctx context.Context, reviewer *db.User, appointments []db.Appointment, mediaID, reason string) (*db.EventMedium, error) {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return nil, ErrMissingReason
	}

	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	event, err := s.repo.Queries().GetEventInstanceByID(ctx, media.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	state, err := s.repo.Queries().GetStateByID(ctx, event.StateID)
	if err != nil {
		return nil, err
	}
	if !s.canModerateMedia(reviewer, appointments, event.StateID, event.LgaID, state.ZoneID) {
		return nil, ErrUnauthorizedReviewer
	}

	// Delete public object from R2 to guarantee complete public eviction
	if media.PublicUrl.Valid && media.PublicUrl.String != "" {
		// Public url contains the key path
		parts := strings.Split(media.PublicUrl.String, "/")
		if len(parts) >= 3 {
			key := strings.Join(parts[len(parts)-4:], "/")
			_ = s.storage.DeleteObject(ctx, key)
		}
	}

	var updated db.EventMedium
	actionID := security.GenerateUUID()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		m, err := q.RemoveEventMedia(ctx, db.RemoveEventMediaParams{
			ID:               mediaID,
			RejectionReason:  TextFromString(cleanReason),
			ReviewedByUserID: TextFromString(reviewer.ID),
		})
		if err != nil {
			return err
		}
		updated = m

		_, err = q.CreateMediaModerationAction(ctx, db.CreateMediaModerationActionParams{
			ID:             actionID,
			MediaID:        mediaID,
			ReviewerUserID: reviewer.ID,
			Action:         "remove",
			Reason:         cleanReason,
			Notes:          TextFromString(cleanReason),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "remove_event_media", "event_media", mediaID, &scopeEvent, &event.ID, &cleanReason, nil)

	return &updated, nil
}

// ToggleHideMedia temporarily hides media during active investigation.
func (s *GalleryService) ToggleHideMedia(ctx context.Context, reviewer *db.User, appointments []db.Appointment, mediaID string, isHidden bool, reason string) (*db.EventMedium, error) {
	media, err := s.repo.Queries().GetEventMediaByID(ctx, mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	event, err := s.repo.Queries().GetEventInstanceByID(ctx, media.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	state, err := s.repo.Queries().GetStateByID(ctx, event.StateID)
	if err != nil {
		return nil, err
	}
	if !s.canModerateMedia(reviewer, appointments, event.StateID, event.LgaID, state.ZoneID) {
		return nil, ErrUnauthorizedReviewer
	}

	action := "hide"
	if !isHidden {
		action = "unhide"
	}

	var updated db.EventMedium
	actionID := security.GenerateUUID()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		m, err := q.ToggleHideEventMedia(ctx, db.ToggleHideEventMediaParams{
			ID:       mediaID,
			IsHidden: isHidden,
		})
		if err != nil {
			return err
		}
		updated = m

		_, err = q.CreateMediaModerationAction(ctx, db.CreateMediaModerationActionParams{
			ID:             actionID,
			MediaID:        mediaID,
			ReviewerUserID: reviewer.ID,
			Action:         action,
			Reason:         reason,
			Notes:          TextFromString(reason),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	scopeEvent := "event"
	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, action+"_event_media", "event_media", mediaID, &scopeEvent, &event.ID, &reason, nil)

	return &updated, nil
}

// Reports & Takedowns Admin
func (s *GalleryService) ListReports(ctx context.Context, status *string, page, limit int32) ([]db.ListMediaReportsRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	items, err := s.repo.Queries().ListMediaReports(ctx, db.ListMediaReportsParams{
		Status: Text(status),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, _ := s.repo.Queries().CountMediaReports(ctx, Text(status))

	if items == nil {
		items = []db.ListMediaReportsRow{}
	}

	return items, total, nil
}

func (s *GalleryService) ResolveReport(ctx context.Context, reviewer *db.User, reportID, status, notes string) (*db.MediaReport, error) {
	cleanStatus := strings.ToLower(strings.TrimSpace(status))
	if cleanStatus != "resolved" && cleanStatus != "dismissed" && cleanStatus != "investigating" {
		return nil, errors.New("invalid report status; allowed: 'investigating', 'resolved', 'dismissed'")
	}

	updated, err := s.repo.Queries().ResolveMediaReport(ctx, db.ResolveMediaReportParams{
		ID:                reportID,
		Status:            cleanStatus,
		ResolutionNotes:   TextFromString(notes),
		ResolvedByUserID:  TextFromString(reviewer.ID),
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "resolve_media_report", "media_report", reportID, nil, nil, &notes, nil)

	return &updated, nil
}

func (s *GalleryService) ListTakedowns(ctx context.Context, status *string, page, limit int32) ([]db.ListMediaTakedownRequestsRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	items, err := s.repo.Queries().ListMediaTakedownRequests(ctx, db.ListMediaTakedownRequestsParams{
		Status: Text(status),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, _ := s.repo.Queries().CountMediaTakedownRequests(ctx, Text(status))

	if items == nil {
		items = []db.ListMediaTakedownRequestsRow{}
	}

	return items, total, nil
}

func (s *GalleryService) ResolveTakedown(ctx context.Context, reviewer *db.User, appointments []db.Appointment, takedownID, decision, reason string) (*db.MediaTakedownRequest, error) {
	cleanDecision := strings.ToLower(strings.TrimSpace(decision))
	if cleanDecision != "approved_removed" && cleanDecision != "rejected_retained" {
		return nil, errors.New("decision must be 'approved_removed' or 'rejected_retained'")
	}

	takedown, err := s.repo.Queries().GetMediaTakedownRequestByID(ctx, takedownID)
	if err != nil {
		return nil, ErrTakedownNotFound
	}

	if cleanDecision == "approved_removed" {
		// Trigger removal of media
		_, _ = s.RemoveMedia(ctx, reviewer, appointments, takedown.MediaID, "Takedown request approved: "+reason)
	}

	updated, err := s.repo.Queries().ResolveMediaTakedownRequest(ctx, db.ResolveMediaTakedownRequestParams{
		ID:               takedownID,
		Status:           cleanDecision,
		ResolutionReason: TextFromString(reason),
		ReviewedByUserID: TextFromString(reviewer.ID),
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &reviewer.ID, &reviewer.PrimaryRole, "resolve_media_takedown", "media_takedown_request", takedownID, nil, nil, &reason, nil)

	return &updated, nil
}

// -------------------------------------------------------------
// Helper: Geographic RBAC
// -------------------------------------------------------------

func (s *GalleryService) canModerateMedia(user *db.User, appointments []db.Appointment, mediaStateID, mediaLgaID, mediaZoneID string) bool {
	if user == nil {
		return false
	}
	if user.PrimaryRole == "super_admin" || user.PrimaryRole == "national_admin" {
		return true
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		if a.RoleType == "super_admin" || a.RoleType == "national_admin" {
			return true
		}
		if a.RoleType == "zonal_coordinator" && a.JurisdictionID.Valid && a.JurisdictionID.String == mediaZoneID {
			return true
		}
		if a.RoleType == "state_coordinator" && a.JurisdictionID.Valid && a.JurisdictionID.String == mediaStateID {
			return true
		}
		if a.RoleType == "lga_representative" && a.JurisdictionID.Valid && a.JurisdictionID.String == mediaLgaID {
			return true
		}
	}

	return false
}

func (s *GalleryService) resolveAdminJurisdiction(user *db.User, appointments []db.Appointment, requestedZone, requestedState, requestedLga *string) (zoneID, stateID, lgaID *string) {
	if user == nil {
		return requestedZone, requestedState, requestedLga
	}
	if user.PrimaryRole == "super_admin" || user.PrimaryRole == "national_admin" {
		return requestedZone, requestedState, requestedLga
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		if a.RoleType == "super_admin" || a.RoleType == "national_admin" {
			return requestedZone, requestedState, requestedLga
		}
		if a.RoleType == "zonal_coordinator" && a.JurisdictionID.Valid {
			zid := a.JurisdictionID.String
			return &zid, requestedState, requestedLga
		}
		if a.RoleType == "state_coordinator" && a.JurisdictionID.Valid {
			sid := a.JurisdictionID.String
			return nil, &sid, requestedLga
		}
		if a.RoleType == "lga_representative" && a.JurisdictionID.Valid {
			lid := a.JurisdictionID.String
			return nil, nil, &lid
		}
	}

	return requestedZone, requestedState, requestedLga
}

func Int4(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}
