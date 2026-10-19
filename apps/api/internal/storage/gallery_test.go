package storage

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func TestValidateGalleryUploadRequest(t *testing.T) {
	// Valid image
	ext, mime, err := ValidateGalleryUploadRequest("cleanup.jpg", "image/jpeg", 1024*1024, "image")
	if err != nil {
		t.Fatalf("expected valid image, got error: %v", err)
	}
	if ext != ".jpg" || mime != "image/jpeg" {
		t.Errorf("expected .jpg / image/jpeg, got %s / %s", ext, mime)
	}

	// Valid video
	ext, mime, err = ValidateGalleryUploadRequest("event.mp4", "video/mp4", 10*1024*1024, "video")
	if err != nil {
		t.Fatalf("expected valid video, got error: %v", err)
	}
	if ext != ".mp4" || mime != "video/mp4" {
		t.Errorf("expected .mp4 / video/mp4, got %s / %s", ext, mime)
	}

	// Image too large (> 10MB)
	_, _, err = ValidateGalleryUploadRequest("huge.jpg", "image/jpeg", 11*1024*1024, "image")
	if err != ErrGalleryImageTooLarge {
		t.Errorf("expected ErrGalleryImageTooLarge, got: %v", err)
	}

	// Video too large (> 100MB)
	_, _, err = ValidateGalleryUploadRequest("huge.mp4", "video/mp4", 101*1024*1024, "video")
	if err != ErrGalleryVideoTooLarge {
		t.Errorf("expected ErrGalleryVideoTooLarge, got: %v", err)
	}

	// Invalid extension
	_, _, err = ValidateGalleryUploadRequest("malware.exe", "image/jpeg", 1024, "image")
	if err != ErrInvalidGalleryImageFormat {
		t.Errorf("expected ErrInvalidGalleryImageFormat, got: %v", err)
	}
}

func TestSanitizeAndReencodeImage(t *testing.T) {
	rawJPEG := createTestJPEG(800, 600)

	sanitized, thumb, w, h, err := SanitizeAndReencodeImage(rawJPEG)
	if err != nil {
		t.Fatalf("SanitizeAndReencodeImage failed: %v", err)
	}

	if w != 800 || h != 600 {
		t.Errorf("expected 800x600, got %dx%d", w, h)
	}

	if len(sanitized) == 0 {
		t.Error("sanitized bytes should not be empty")
	}

	if len(thumb) == 0 {
		t.Error("thumbnail bytes should not be empty")
	}

	// Verify decoded thumbnail preserves aspect ratio
	thumbCfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("failed to decode thumbnail: %v", err)
	}
	if thumbCfg.Width > 480 || thumbCfg.Height > 480 {
		t.Errorf("thumbnail dimensions %dx%d exceed 480x480 bounding box", thumbCfg.Width, thumbCfg.Height)
	}

	// Verify APP1 marker (EXIF: 0xFF 0xE1) is absent in re-encoded JPEG output
	if bytes.Contains(sanitized, []byte{0xFF, 0xE1}) {
		t.Error("re-encoded JPEG must not contain EXIF APP1 marker")
	}
}

func TestGalleryKeyGenerators(t *testing.T) {
	pending := GenerateGalleryPendingKey("image", "jpg")
	if !strings.HasPrefix(pending, GalleryQuarantinePrefix) {
		t.Errorf("expected pending key with prefix %s, got %s", GalleryQuarantinePrefix, pending)
	}

	pub := GenerateGalleryPublicKey("event-123", "image", "jpg")
	if !strings.HasPrefix(pub, "gallery/events/event-123/images/") {
		t.Errorf("expected public key with event prefix, got %s", pub)
	}

	thumb := GenerateGalleryThumbnailKey("media-456")
	if !strings.HasPrefix(thumb, GalleryThumbnailsPrefix) {
		t.Errorf("expected thumb key with prefix %s, got %s", GalleryThumbnailsPrefix, thumb)
	}
}
