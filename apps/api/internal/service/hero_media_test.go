package service_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func createTestUserWithRole(t *testing.T, env *TestEnv, role, email string) *db.User {
	t.Helper()
	ctx := context.Background()
	hash, err := security.HashPassword("TestPass123!")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userID := security.GenerateUUID()
	user, err := env.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               service.Text(&email),
		PasswordHash:        hash,
		DisplayName:         fmt.Sprintf("User %s", role),
		MembershipReference: fmt.Sprintf("REF-%s-%d", role, time.Now().UnixNano()),
		Status:              "active",
		PrimaryRole:         role,
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("failed to create test user with role %s: %v", role, err)
	}
	return &user
}

func TestHeroMedia_AdminAuthorization(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	member := createTestUserWithRole(t, env, "member", fmt.Sprintf("member_%d@ratels.org", time.Now().UnixNano()))
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	// Member attempt -> must be rejected with ErrUnauthorizedMediaAction
	_, err := env.HeroMediaService.AuthorizeHeroVideoUpload(ctx, member, "hero.mp4", "video/mp4", 10*1024*1024, "Member Hero")
	if err != service.ErrUnauthorizedMediaAction {
		t.Fatalf("expected ErrUnauthorizedMediaAction for member, got %v", err)
	}

	_, err = env.HeroMediaService.ListHeroMedia(ctx, member)
	if err != service.ErrUnauthorizedMediaAction {
		t.Fatalf("expected ErrUnauthorizedMediaAction for member listing media, got %v", err)
	}

	err = env.HeroMediaService.DeleteHeroMedia(ctx, member, "some-id")
	if err != service.ErrUnauthorizedMediaAction {
		t.Fatalf("expected ErrUnauthorizedMediaAction for member deleting media, got %v", err)
	}

	// Super Admin attempt -> authorized
	presignRes, err := env.HeroMediaService.AuthorizeHeroVideoUpload(ctx, superAdmin, "hero.mp4", "video/mp4", 10*1024*1024, "Admin Hero")
	if err != nil {
		t.Fatalf("super_admin upload authorization failed: %v", err)
	}
	if presignRes.UploadURL == "" || presignRes.ObjectKey == "" || presignRes.PublicURL == "" {
		t.Errorf("presign response missing required fields: %+v", presignRes)
	}
}

func TestHeroMedia_PresignedUploadAndPublish(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	// 1. Initially, if no active video exists, GetActiveHeroMedia returns active: false
	// Ensure clean state by toggling off any existing active media
	_, _ = env.HeroMediaService.ToggleHeroMedia(ctx, superAdmin, false)

	activeCheck, err := env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil {
		t.Fatalf("failed to fetch initial active hero media: %v", err)
	}
	if activeCheck.Active {
		t.Fatalf("expected active to be false when no media active, got true")
	}
	if activeCheck.Media != nil {
		t.Fatalf("expected media to be nil when no media active")
	}

	// 2. Authorize presigned upload
	presign, err := env.HeroMediaService.AuthorizeHeroVideoUpload(ctx, superAdmin, "national_movement.mp4", "video/mp4", 12*1024*1024, "National Movement Hero")
	if err != nil {
		t.Fatalf("failed to authorize presigned upload: %v", err)
	}

	// Active status should still be false because upload is not yet published!
	activeCheck, err = env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil {
		t.Fatalf("failed to fetch active hero media: %v", err)
	}
	if activeCheck.Active {
		t.Fatalf("unpublished upload should not be active on public homepage")
	}

	// 3. Explicitly Publish the video
	published, err := env.HeroMediaService.PublishHeroMedia(ctx, superAdmin, presign.ID)
	if err != nil {
		t.Fatalf("failed to publish hero media: %v", err)
	}
	if !published.IsActive {
		t.Errorf("expected published media to have IsActive = true")
	}
	if published.PublishedAt == nil {
		t.Errorf("expected PublishedAt timestamp to be set")
	}

	// 4. Public endpoint now returns the published video
	activeCheck, err = env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil {
		t.Fatalf("failed to fetch active hero media after publish: %v", err)
	}
	if !activeCheck.Active || activeCheck.Media == nil {
		t.Fatalf("expected public endpoint to return active media, got active=%v, media=%+v", activeCheck.Active, activeCheck.Media)
	}
	if activeCheck.Media.ID != presign.ID {
		t.Errorf("expected active media ID %s, got %s", presign.ID, activeCheck.Media.ID)
	}
	if activeCheck.Media.VideoURL != presign.PublicURL {
		t.Errorf("expected video URL %s, got %s", presign.PublicURL, activeCheck.Media.VideoURL)
	}
}

func TestHeroMedia_DirectUploadAndAtomicReplacement(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	natAdmin := createTestUserWithRole(t, env, "national_admin", fmt.Sprintf("nat_%d@ratels.org", time.Now().UnixNano()))

	// Create simulated MP4 bytes
	mp4Bytes := make([]byte, 2048)
	copy(mp4Bytes[4:8], []byte("ftyp"))
	copy(mp4Bytes[8:12], []byte("mp42"))

	// 1. Upload Video A
	videoA, err := env.HeroMediaService.UploadHeroVideoDirect(ctx, natAdmin, "campaign_a.mp4", "Campaign A Video", bytes.NewReader(mp4Bytes), int64(len(mp4Bytes)))
	if err != nil {
		t.Fatalf("failed to upload video A: %v", err)
	}

	// 2. Publish Video A
	publishedA, err := env.HeroMediaService.PublishHeroMedia(ctx, natAdmin, videoA.ID)
	if err != nil {
		t.Fatalf("failed to publish video A: %v", err)
	}
	if !publishedA.IsActive {
		t.Errorf("expected video A to be active")
	}

	// 3. Upload Video B
	videoB, err := env.HeroMediaService.UploadHeroVideoDirect(ctx, natAdmin, "campaign_b.mp4", "Campaign B Video", bytes.NewReader(mp4Bytes), int64(len(mp4Bytes)))
	if err != nil {
		t.Fatalf("failed to upload video B: %v", err)
	}

	// 4. Publish Video B (Atomic replacement)
	publishedB, err := env.HeroMediaService.PublishHeroMedia(ctx, natAdmin, videoB.ID)
	if err != nil {
		t.Fatalf("failed to publish video B: %v", err)
	}
	if !publishedB.IsActive {
		t.Errorf("expected video B to be active")
	}

	// 5. Verify Video A is now inactive
	allMedia, err := env.HeroMediaService.ListHeroMedia(ctx, natAdmin)
	if err != nil {
		t.Fatalf("failed to list media: %v", err)
	}
	for _, m := range allMedia {
		if m.ID == videoA.ID && m.IsActive {
			t.Errorf("video A must be deactivated when video B was published")
		}
		if m.ID == videoB.ID && !m.IsActive {
			t.Errorf("video B must be active")
		}
	}

	// 6. Public endpoint reflects Video B
	active, err := env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil || !active.Active {
		t.Fatalf("failed to get active media: %v", err)
	}
	if active.Media.ID != videoB.ID {
		t.Errorf("expected active media to be video B, got %s", active.Media.ID)
	}
}

func TestHeroMedia_TogglePlaybackAndFallback(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	mp4Bytes := make([]byte, 2048)
	copy(mp4Bytes[4:8], []byte("ftyp"))

	video, err := env.HeroMediaService.UploadHeroVideoDirect(ctx, superAdmin, "toggle_test.mp4", "Toggle Test Video", bytes.NewReader(mp4Bytes), int64(len(mp4Bytes)))
	if err != nil {
		t.Fatalf("failed to upload video: %v", err)
	}

	_, err = env.HeroMediaService.PublishHeroMedia(ctx, superAdmin, video.ID)
	if err != nil {
		t.Fatalf("failed to publish video: %v", err)
	}

	// 1. Disable video playback -> default green gradient active
	toggleRes, err := env.HeroMediaService.ToggleHeroMedia(ctx, superAdmin, false)
	if err != nil {
		t.Fatalf("failed to toggle video playback off: %v", err)
	}
	if toggleRes.Active {
		t.Errorf("expected toggle result active to be false")
	}

	// Verify public endpoint returns active: false
	pub, err := env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil {
		t.Fatalf("failed to get active media: %v", err)
	}
	if pub.Active {
		t.Errorf("expected public endpoint active to be false after toggle off")
	}

	// 2. Re-enable video playback -> re-activates latest published video
	toggleOn, err := env.HeroMediaService.ToggleHeroMedia(ctx, superAdmin, true)
	if err != nil {
		t.Fatalf("failed to toggle video playback on: %v", err)
	}
	if !toggleOn.Active || toggleOn.Media == nil {
		t.Fatalf("expected toggle on to re-activate media, got active=%v, media=%+v", toggleOn.Active, toggleOn.Media)
	}

	pub, err = env.HeroMediaService.GetActiveHeroMedia(ctx)
	if err != nil || !pub.Active {
		t.Fatalf("expected public endpoint active to be true after toggle on: %v", err)
	}
}

func TestHeroMedia_PosterAndDeletion(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	mp4Bytes := make([]byte, 2048)
	copy(mp4Bytes[4:8], []byte("ftyp"))

	video, err := env.HeroMediaService.UploadHeroVideoDirect(ctx, superAdmin, "poster_test.mp4", "Poster Test Video", bytes.NewReader(mp4Bytes), int64(len(mp4Bytes)))
	if err != nil {
		t.Fatalf("failed to upload video: %v", err)
	}

	// 1. Upload Poster (1x1 PNG image)
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG magic
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, // 8-bit RGBA
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, // IDAT
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, // IEND
		0x42, 0x60, 0x82,
	}

	updated, err := env.HeroMediaService.UploadHeroPoster(ctx, superAdmin, video.ID, "poster.png", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	if err != nil {
		t.Fatalf("failed to upload poster: %v", err)
	}
	if updated.PosterURL == "" || updated.PosterKey == "" {
		t.Errorf("expected poster key and URL to be set: %+v", updated)
	}

	// 2. Delete the hero video
	err = env.HeroMediaService.DeleteHeroMedia(ctx, superAdmin, video.ID)
	if err != nil {
		t.Fatalf("failed to delete hero media: %v", err)
	}

	// Confirm not found
	all, err := env.HeroMediaService.ListHeroMedia(ctx, superAdmin)
	if err != nil {
		t.Fatalf("failed to list media: %v", err)
	}
	for _, m := range all {
		if m.ID == video.ID {
			t.Errorf("deleted media %s still found in list", video.ID)
		}
	}
}
