package service_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"image"
	"image/color"
	"image/jpeg"
	"ratels-api/internal/security"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

func createTestJPEGBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func createTestCleanupEvent(t *testing.T, env *TestEnv, creatorID, stateID, lgaID string) *db.EventInstance {
	t.Helper()
	ctx := context.Background()

	// Event creation requires an appointed coordinator.
	// The uploader remains an ordinary member.
	_ = creatorID
	regCoord, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("gallery_coord_%d@example.org", time.Now().UnixNano()),
		Password:    "TestPassword123!",
		DisplayName: "Gallery Event Coordinator",
	})
	if err != nil {
		t.Fatalf("Failed to register gallery coordinator: %v", err)
	}

	// Operational appointments require an active, verified account.
	if _, err := env.Pool.Exec(ctx, `UPDATE users SET status='active',is_verified=true WHERE id=$1`, regCoord.UserID); err != nil {
		t.Fatal(err)
	}

	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           regCoord.UserID,
		RoleTitle:        "Gallery State Coordinator",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateID),
		StartDate: pgtype.Timestamptz{
			Time:  time.Now().Add(-time.Hour),
			Valid: true,
		},
		Status: "active",
	})
	if err != nil {
		t.Fatalf("Failed to appoint gallery coordinator: %v", err)
	}

	event, err := env.CleanupService.CreateEvent(ctx, regCoord.UserID, "state_coordinator", nil, service.CreateEventRequest{
		Title:         fmt.Sprintf("Cleanup Event %d", time.Now().UnixNano()),
		StateID:       stateID,
		LGAID:         lgaID,
		Address:       "Victoria Island Beachfront",
		Landmark:      "Bar Beach",
		StartTime:     time.Now().Add(-4 * time.Hour),
		EndTime:       time.Now().Add(-1 * time.Hour),
		Capacity:      150,
		PublicContact: "cleanup@ratels.org",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create test cleanup event: %v", err)
	}
	return event
}

func TestGalleryService_UploadAuthorizationAndValidation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, err := env.GeoService.ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("No states found: %v", err)
	}
	state := states[0]
	lgas, err := env.GeoService.ListLGAsByState(ctx, state.ID)
	if err != nil || len(lgas) == 0 {
		t.Fatalf("No LGAs found: %v", err)
	}
	lga := lgas[0]

	member := createTestUserWithRole(t, env, "member", fmt.Sprintf("uploader_%d@ratels.org", time.Now().UnixNano()))
	event := createTestCleanupEvent(t, env, member.ID, state.ID, lga.ID)

	// 1. Valid image upload presign
	authRes, err := env.GalleryService.AuthorizeUpload(ctx, member, service.AuthorizeGalleryUploadRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Filename:      "volunteer_cleanup.jpg",
		ContentType:   "image/jpeg",
		FileSizeBytes: 2 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("Failed to authorize valid image upload: %v", err)
	}
	if authRes.UploadURL == "" || authRes.StorageKey == "" {
		t.Errorf("AuthorizeUpload returned empty upload url or storage key: %+v", authRes)
	}

	// 2. Reject non-eligible event (e.g. non-existent)
	_, err = env.GalleryService.AuthorizeUpload(ctx, member, service.AuthorizeGalleryUploadRequest{
		EventID:       "invalid-event-id",
		MediaType:     "image",
		Filename:      "pic.jpg",
		ContentType:   "image/jpeg",
		FileSizeBytes: 1024 * 1024,
	})
	if err == nil {
		t.Fatalf("Expected error for non-existent event, got nil")
	}

	// 3. Reject file exceeding 10MB limit
	_, err = env.GalleryService.AuthorizeUpload(ctx, member, service.AuthorizeGalleryUploadRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Filename:      "huge.jpg",
		ContentType:   "image/jpeg",
		FileSizeBytes: 11 * 1024 * 1024,
	})
	if err != storage.ErrGalleryImageTooLarge {
		t.Fatalf("Expected ErrGalleryImageTooLarge, got %v", err)
	}
}

func TestGalleryService_ConsentAndSubmissionSafeguards(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, _ := env.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := env.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	member := createTestUserWithRole(t, env, "member", fmt.Sprintf("uploader_%d@ratels.org", time.Now().UnixNano()))
	event := createTestCleanupEvent(t, env, member.ID, state.ID, lga.ID)

	// 1. Missing mandatory consent declaration -> ErrConsentRequired
	_, err := env.GalleryService.SubmitMedia(ctx, member, service.SubmitGalleryMediaRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Caption:       "Volunteers clearing beach plastic",
		StorageKey:    "gallery/quarantine/image/testkey.jpg",
		MimeType:      "image/jpeg",
		FileSizeBytes: 500 * 1024,
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       false, // missing!
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
		},
	})
	if err != service.ErrConsentRequired {
		t.Fatalf("Expected ErrConsentRequired, got %v", err)
	}

	// 2. Minors pictured without guardian consent -> ErrGuardianConsentRequired
	_, err = env.GalleryService.SubmitMedia(ctx, member, service.SubmitGalleryMediaRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Caption:       "Youth cleanup ambassadors",
		StorageKey:    "gallery/quarantine/image/testkey.jpg",
		MimeType:      "image/jpeg",
		FileSizeBytes: 500 * 1024,
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       true,
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
			HasMinors:                true,
			HasGuardianConsent:       false, // missing guardian consent!
		},
	})
	if err != service.ErrGuardianConsentRequired {
		t.Fatalf("Expected ErrGuardianConsentRequired, got %v", err)
	}

	// 3. Valid full consent -> successfully recorded as pending_review
	media, err := env.GalleryService.SubmitMedia(ctx, member, service.SubmitGalleryMediaRequest{
		EventID:                event.ID,
		MediaType:              "image",
		Caption:                "Youth cleanup ambassadors",
		StorageKey:             "gallery/quarantine/image/testkey.jpg",
		MimeType:               "image/jpeg",
		FileSizeBytes:          500 * 1024,
		AllowPublicAttribution: true,
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       true,
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
			HasMinors:                true,
			HasGuardianConsent:       true,
		},
	})
	if err != nil {
		t.Fatalf("Failed to submit valid media: %v", err)
	}
	if media.ModerationStatus != "pending_review" {
		t.Errorf("Expected moderation status 'pending_review', got '%s'", media.ModerationStatus)
	}
	if media.IsPublished {
		t.Errorf("Pending media must not be published")
	}
	if media.PublicUrl.Valid {
		t.Errorf("Pending media must not have a public URL")
	}
}

func TestGalleryService_ModerationLifecycleAndAntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, _ := env.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := env.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	uploader := createTestUserWithRole(t, env, "member", fmt.Sprintf("uploader_%d@ratels.org", time.Now().UnixNano()))
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))
	otherMember := createTestUserWithRole(t, env, "member", fmt.Sprintf("other_%d@ratels.org", time.Now().UnixNano()))

	event := createTestCleanupEvent(t, env, superAdmin.ID, state.ID, lga.ID)

	// Create a real quarantine image object in storage
	quarantineKey := storage.GenerateGalleryPendingKey("image", "jpg")
	rawJPEG := createTestJPEGBytes(640, 480)
	_, err := env.MockStorage.UploadObject(ctx, quarantineKey, "image/jpeg", bytes.NewReader(rawJPEG), int64(len(rawJPEG)))
	if err != nil {
		t.Fatalf("Failed to mock upload quarantine image: %v", err)
	}

	media, err := env.GalleryService.SubmitMedia(ctx, uploader, service.SubmitGalleryMediaRequest{
		EventID:                event.ID,
		MediaType:              "image",
		Caption:                "Recycling plastic bins deployment",
		StorageKey:             quarantineKey,
		MimeType:               "image/jpeg",
		FileSizeBytes:          int64(len(rawJPEG)),
		AllowPublicAttribution: true,
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       true,
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
		},
	})
	if err != nil {
		t.Fatalf("Failed to submit media: %v", err)
	}

	// 1. Anti-Self-Approval: uploader cannot approve their own media
	_, err = env.GalleryService.ApproveMedia(ctx, uploader, nil, media.ID, nil)
	if err != service.ErrSelfApprovalNotAllowed {
		t.Fatalf("Expected ErrSelfApprovalNotAllowed for uploader approving own media, got %v", err)
	}

	// 2. Unauthorized reviewer: ordinary member cannot approve
	_, err = env.GalleryService.ApproveMedia(ctx, otherMember, nil, media.ID, nil)
	if err != service.ErrUnauthorizedReviewer {
		t.Fatalf("Expected ErrUnauthorizedReviewer for non-admin, got %v", err)
	}

	// 3. Super Admin approval: succeeds, runs metadata sanitization, generates public derivatives
	notes := "Verified authentic cleanup photo"
	approved, err := env.GalleryService.ApproveMedia(ctx, superAdmin, nil, media.ID, &notes)
	if err != nil {
		t.Fatalf("Super admin approval failed: %v", err)
	}
	if approved.ModerationStatus != "approved" {
		t.Errorf("Expected status 'approved', got '%s'", approved.ModerationStatus)
	}
	if !approved.IsPublished {
		t.Errorf("Approved media should be marked is_published = true")
	}
	if !approved.MetadataSanitized {
		t.Errorf("Approved media should be marked metadata_sanitized = true")
	}
	if !approved.PublicUrl.Valid || approved.PublicUrl.String == "" {
		t.Errorf("Approved media must have a public URL")
	}
	if !approved.ThumbnailUrl.Valid || approved.ThumbnailUrl.String == "" {
		t.Errorf("Approved media must have a thumbnail URL")
	}

	// 4. Public gallery query should now include this approved media
	pubRes, err := env.GalleryService.ListPublicGallery(ctx, service.PublicGalleryFilter{
		EventID: &event.ID,
		Page:    1,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Failed to list public gallery: %v", err)
	}
	if pubRes.Total < 1 {
		t.Fatalf("Expected at least 1 public media, got %d", pubRes.Total)
	}
	found := false
	for _, item := range pubRes.Items {
		if item.ID == media.ID {
			found = true
			if item.UploaderDisplayName != uploader.DisplayName {
				t.Errorf("Expected uploader attribution '%s', got '%v'", uploader.DisplayName, item.UploaderDisplayName)
			}
			break
		}
	}
	if !found {
		t.Errorf("Approved media %s was not found in public gallery response", media.ID)
	}

	// 5. Temporary hide toggle
	hidden, err := env.GalleryService.ToggleHideMedia(ctx, superAdmin, nil, media.ID, true, "Temporarily hiding for inspection")
	if err != nil {
		t.Fatalf("Toggle hide media failed: %v", err)
	}
	if !hidden.IsHidden {
		t.Errorf("Expected is_hidden = true")
	}

	// Now it must NOT appear in public gallery!
	pubRes2, _ := env.GalleryService.ListPublicGallery(ctx, service.PublicGalleryFilter{
		EventID: &event.ID,
		Page:    1,
		Limit:   10,
	})
	for _, item := range pubRes2.Items {
		if item.ID == media.ID {
			t.Errorf("Hidden media should not appear in public gallery")
		}
	}

	// Unhide it
	_, _ = env.GalleryService.ToggleHideMedia(ctx, superAdmin, nil, media.ID, false, "Inspection cleared")

	// 6. Complete removal
	removed, err := env.GalleryService.RemoveMedia(ctx, superAdmin, nil, media.ID, "Removed per admin decision")
	if err != nil {
		t.Fatalf("Remove media failed: %v", err)
	}
	if removed.ModerationStatus != "removed" || !removed.IsHidden || removed.IsPublished {
		t.Errorf("Removed media should be status 'removed', hidden=true, is_published=false")
	}
}

func TestGalleryService_ReportingAndTakedownWorkflow(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, _ := env.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := env.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	uploader := createTestUserWithRole(t, env, "member", fmt.Sprintf("uploader_%d@ratels.org", time.Now().UnixNano()))
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))
	event := createTestCleanupEvent(t, env, superAdmin.ID, state.ID, lga.ID)

	quarantineKey := storage.GenerateGalleryPendingKey("image", "jpg")
	rawJPEG := createTestJPEGBytes(640, 480)
	_, _ = env.MockStorage.UploadObject(ctx, quarantineKey, "image/jpeg", bytes.NewReader(rawJPEG), int64(len(rawJPEG)))

	media, _ := env.GalleryService.SubmitMedia(ctx, uploader, service.SubmitGalleryMediaRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Caption:       "Community cleanup group",
		StorageKey:    quarantineKey,
		MimeType:      "image/jpeg",
		FileSizeBytes: int64(len(rawJPEG)),
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       true,
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
		},
	})

	_, _ = env.GalleryService.ApproveMedia(ctx, superAdmin, nil, media.ID, nil)

	// 1. Public Visitor submits a report
	email := "concerned_citizen@example.com"
	report, err := env.GalleryService.ReportMedia(ctx, nil, media.ID, service.CreateMediaReportRequest{
		ReporterEmail: &email,
		ReportReason:  "privacy_violation",
		Details:       "My family member is pictured without prior written notice.",
	})
	if err != nil {
		t.Fatalf("Failed to report media: %v", err)
	}
	if report.Status != "pending" {
		t.Errorf("Expected report status 'pending', got '%s'", report.Status)
	}

	// 2. Admin lists reports and resolves
	reports, total, err := env.GalleryService.ListReports(ctx, nil, 1, 10)
	if err != nil || total < 1 {
		t.Fatalf("Expected pending reports, got total: %d, err: %v", total, err)
	}
	_ = reports

	resolvedReport, err := env.GalleryService.ResolveReport(ctx, superAdmin, report.ID, "resolved", "Investigated and verified consent")
	if err != nil {
		t.Fatalf("Failed to resolve report: %v", err)
	}
	if resolvedReport.Status != "resolved" {
		t.Errorf("Expected report status 'resolved', got '%s'", resolvedReport.Status)
	}

	// 3. Formal takedown request submission
	takedown, err := env.GalleryService.SubmitTakedownRequest(ctx, nil, media.ID, service.CreateMediaTakedownRequest{
		RequesterName:       "John Doe",
		RequesterEmail:      "john@example.com",
		RelationshipToMedia: "subject_in_media",
		Reason:              "privacy_violation",
		Details:             "I revoke my consent to appear in this public activity photograph.",
	})
	if err != nil {
		t.Fatalf("Failed to submit takedown request: %v", err)
	}
	if takedown.Status != "pending" {
		t.Errorf("Expected takedown status 'pending', got '%s'", takedown.Status)
	}

	// 4. Admin resolves takedown with approved_removed -> automatically triggers removal!
	resTakedown, err := env.GalleryService.ResolveTakedown(ctx, superAdmin, nil, takedown.ID, "approved_removed", "Consent withdrawn by subject")
	if err != nil {
		t.Fatalf("Failed to resolve takedown: %v", err)
	}
	if resTakedown.Status != "approved_removed" {
		t.Errorf("Expected takedown status 'approved_removed', got '%s'", resTakedown.Status)
	}

	// Verify media is now removed!
	checkMedia, err := env.Repo.Queries().GetEventMediaByID(ctx, media.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve media: %v", err)
	}
	if checkMedia.ModerationStatus != "removed" || !checkMedia.IsHidden || checkMedia.IsPublished {
		t.Errorf("Expected media to be removed and hidden after approved takedown, got status: %s, hidden: %v, published: %v",
			checkMedia.ModerationStatus, checkMedia.IsHidden, checkMedia.IsPublished)
	}
}

func TestGalleryService_WithdrawalAndRevisionFlow(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, _ := env.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := env.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	uploader := createTestUserWithRole(t, env, "member", fmt.Sprintf("uploader_%d@ratels.org", time.Now().UnixNano()))
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))
	event := createTestCleanupEvent(t, env, superAdmin.ID, state.ID, lga.ID)

	media, _ := env.GalleryService.SubmitMedia(ctx, uploader, service.SubmitGalleryMediaRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Caption:       "Draft street cleaning image",
		StorageKey:    "gallery/quarantine/image/draft.jpg",
		MimeType:      "image/jpeg",
		FileSizeBytes: 100 * 1024,
		Consent: service.GalleryConsentInput{
			HasOwnerPermission:       true,
			UnderstandsPublicDisplay: true,
			HasIdentifiableConsent:   true,
			HasNoConfidentialInfo:    true,
		},
	})

	// 1. Admin requests revision
	revised, err := env.GalleryService.RequestRevisionMedia(ctx, superAdmin, nil, media.ID, "Please provide a more descriptive caption")
	if err != nil {
		t.Fatalf("Request revision failed: %v", err)
	}
	if revised.ModerationStatus != "revision_requested" {
		t.Errorf("Expected status 'revision_requested', got '%s'", revised.ModerationStatus)
	}

	// 2. Uploader decides to withdraw
	withdrawn, err := env.GalleryService.WithdrawSubmission(ctx, uploader, media.ID, "Will re-submit a better photo")
	if err != nil {
		t.Fatalf("Withdraw submission failed: %v", err)
	}
	if withdrawn.ModerationStatus != "withdrawn" {
		t.Errorf("Expected status 'withdrawn', got '%s'", withdrawn.ModerationStatus)
	}
}
