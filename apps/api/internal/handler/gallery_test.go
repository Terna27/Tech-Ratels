package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

func createTestEventForGallery(t *testing.T, h *TestHarness, creatorID, stateID, lgaID string) *db.EventInstance {
	t.Helper()
	event, err := h.CleanupService.CreateEvent(context.Background(), creatorID, "national_admin", nil, service.CreateEventRequest{
		Title:         fmt.Sprintf("Cleanup Gallery Event %d", time.Now().UnixNano()),
		StateID:       stateID,
		LGAID:         lgaID,
		Address:       "Beach Road",
		Landmark:      "City Gate",
		StartTime:     time.Now().Add(-5 * time.Hour),
		EndTime:       time.Now().Add(-2 * time.Hour),
		Capacity:      200,
		PublicContact: "contact@ratels.org",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create test cleanup event: %v", err)
	}
	return event
}

func TestGalleryHTTP_PublicEndpointsAndFiltering(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. Fetch public gallery (should return 200 with empty items list initially)
	req, _ := http.NewRequest("GET", "/api/v1/gallery", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GET /api/v1/gallery, got %d: %s", rr.Code, rr.Body.String())
	}

	var pubResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &pubResp); err != nil {
		t.Fatalf("Failed to parse gallery response: %v", err)
	}
	if _, ok := pubResp["items"]; !ok {
		t.Errorf("Expected 'items' array in gallery response: %+v", pubResp)
	}

	// 2. Fetch featured gallery
	req, _ = http.NewRequest("GET", "/api/v1/gallery/featured", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GET /api/v1/gallery/featured, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Report a non-existent media item -> 404
	reportBody, _ := json.Marshal(map[string]interface{}{
		"report_reason": "privacy_violation",
		"details":       "Testing reporting non-existent media",
	})
	req, _ = http.NewRequest("POST", "/api/v1/gallery/non-existent-id/report", bytes.NewReader(reportBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for non-existent media report, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGalleryHTTP_MemberUploadAndSubmissionFlow(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	states, _ := h.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := h.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	member, token := createTestUserWithToken(t, h, "gal_mem", "Gallery Member", "member")
	superAdmin, saToken := createTestUserWithToken(t, h, "gal_sa", "Gallery SuperAdmin", "super_admin")
	event := createTestEventForGallery(t, h, superAdmin.ID, state.ID, lga.ID)

	// 1. Authorize Upload presign
	presignBody, _ := json.Marshal(map[string]interface{}{
		"event_id":        event.ID,
		"media_type":      "image",
		"filename":        "coastal_cleanup.jpg",
		"content_type":    "image/jpeg",
		"file_size_bytes": 1024 * 1024,
	})
	req, _ := http.NewRequest("POST", "/api/v1/me/gallery/presign", bytes.NewReader(presignBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from presign, got %d: %s", rr.Code, rr.Body.String())
	}
	var presignRes map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &presignRes)
	storageKey := presignRes["storage_key"].(string)

	// Mock file upload into quarantine storage
	rawJPEG := makeTestJPEGBytes(800, 600)
	_, _ = h.Storage.UploadObject(ctx, storageKey, "image/jpeg", bytes.NewReader(rawJPEG), int64(len(rawJPEG)))

	// 2. Submit media with full consent declaration
	submitBody, _ := json.Marshal(map[string]interface{}{
		"event_id":                  event.ID,
		"media_type":                "image",
		"caption":                   "Volunteers gathering recyclable waste",
		"storage_key":               storageKey,
		"mime_type":                 "image/jpeg",
		"file_size_bytes":           len(rawJPEG),
		"allow_public_attribution":  true,
		"consent": map[string]interface{}{
			"has_owner_permission":        true,
			"understands_public_display":  true,
			"has_identifiable_consent":    true,
			"has_no_confidential_info":     true,
			"has_minors":                  false,
			"has_guardian_consent":        false,
		},
	})
	req, _ = http.NewRequest("POST", "/api/v1/me/gallery/submit", bytes.NewReader(submitBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from submit, got %d: %s", rr.Code, rr.Body.String())
	}

	var mediaItem map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &mediaItem)
	mediaID := mediaItem["id"].(string)

	// 3. Verify media is listed in member's own submissions
	req, _ = http.NewRequest("GET", "/api/v1/me/gallery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GET /api/v1/me/gallery, got %d: %s", rr.Code, rr.Body.String())
	}
	var mySubs map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &mySubs)
	if mySubs["total"].(float64) < 1 {
		t.Errorf("Expected at least 1 submission in my submissions, got %v", mySubs["total"])
	}

	// 4. Pending media MUST NOT be visible in public gallery!
	req, _ = http.NewRequest("GET", "/api/v1/gallery/"+mediaID, nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 from public endpoint for unapproved media, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Admin can inspect media detail in admin review workspace
	req, _ = http.NewRequest("GET", "/api/v1/admin/gallery/"+mediaID, nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from admin detail, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Member cannot approve media -> 403 Forbidden
	approveBody, _ := json.Marshal(map[string]interface{}{"notes": "Self approval attempt"})
	req, _ = http.NewRequest("POST", "/api/v1/admin/gallery/"+mediaID+"/approve", bytes.NewReader(approveBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing admin approve, got %d", rr.Code)
	}

	// 7. Super Admin approves media
	req, _ = http.NewRequest("POST", "/api/v1/admin/gallery/"+mediaID+"/approve", bytes.NewReader(approveBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from admin approve, got %d: %s", rr.Code, rr.Body.String())
	}

	// 8. Approved media is now visible in public gallery!
	req, _ = http.NewRequest("GET", "/api/v1/gallery/"+mediaID, nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from public detail after approval, got %d: %s", rr.Code, rr.Body.String())
	}

	// 9. Admin overview metrics reflect the approved media
	req, _ = http.NewRequest("GET", "/api/v1/admin/gallery/overview", nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from admin overview, got %d: %s", rr.Code, rr.Body.String())
	}
	var overviewRes map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &overviewRes)
	if overviewRes["approved_count"].(float64) < 1 {
		t.Errorf("Expected approved_count >= 1, got %v", overviewRes["approved_count"])
	}

	_ = member
}

func TestGalleryHTTP_ReportAndTakedownResolution(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	states, _ := h.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := h.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	uploader, _ := createTestUserWithToken(t, h, "uploader", "Uploader User", "member")
	superAdmin, saToken := createTestUserWithToken(t, h, "sa_take", "SA Takedown", "super_admin")
	event := createTestEventForGallery(t, h, superAdmin.ID, state.ID, lga.ID)

	quarantineKey := storage.GenerateGalleryPendingKey("image", "jpg")
	rawJPEG := makeTestJPEGBytes(600, 400)
	_, _ = h.Storage.UploadObject(ctx, quarantineKey, "image/jpeg", bytes.NewReader(rawJPEG), int64(len(rawJPEG)))

	media, _ := h.GalleryService.SubmitMedia(ctx, uploader, service.SubmitGalleryMediaRequest{
		EventID:       event.ID,
		MediaType:     "image",
		Caption:       "Community cleanup in progress",
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
	_, _ = h.GalleryService.ApproveMedia(ctx, superAdmin, nil, media.ID, nil)

	// 1. Submit public takedown request
	takedownPayload, _ := json.Marshal(map[string]interface{}{
		"requester_name":        "Alice Smith",
		"requester_email":       "alice@example.org",
		"relationship_to_media": "subject_in_media",
		"reason":                "privacy_violation",
		"details":               "Photo taken without my consent",
	})
	req, _ := http.NewRequest("POST", "/api/v1/gallery/"+media.ID+"/takedown", bytes.NewReader(takedownPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from submit takedown, got %d: %s", rr.Code, rr.Body.String())
	}
	var tdRes map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &tdRes)
	takedownID := tdRes["id"].(string)

	// 2. Admin lists takedowns
	req, _ = http.NewRequest("GET", "/api/v1/admin/gallery/takedowns", nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GET takedowns, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Admin resolves takedown with approved_removed
	resolvePayload, _ := json.Marshal(map[string]interface{}{
		"decision": "approved_removed",
		"reason":   "Consent verified as lacking",
	})
	req, _ = http.NewRequest("POST", "/api/v1/admin/gallery/takedowns/"+takedownID+"/resolve", bytes.NewReader(resolvePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from resolve takedown, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Verify media is no longer accessible via public API
	req, _ = http.NewRequest("GET", "/api/v1/gallery/"+media.ID, nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 from public gallery after takedown removal, got %d", rr.Code)
	}
}
