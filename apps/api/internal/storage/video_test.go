package storage

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateHeroVideoUploadRequest(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		contentType   string
		fileSize      int64
		wantErr       bool
		expectedExt   string
		expectedMime  string
		expectedError error
	}{
		{
			name:         "Valid MP4",
			filename:     "hero_cinematic.mp4",
			contentType:  "video/mp4",
			fileSize:     15 * 1024 * 1024,
			wantErr:      false,
			expectedExt:  ".mp4",
			expectedMime: "video/mp4",
		},
		{
			name:         "Valid WebM",
			filename:     "national_cleanup.webm",
			contentType:  "video/webm",
			fileSize:     8 * 1024 * 1024,
			wantErr:      false,
			expectedExt:  ".webm",
			expectedMime: "video/webm",
		},
		{
			name:          "Empty file",
			filename:      "empty.mp4",
			contentType:   "video/mp4",
			fileSize:      0,
			wantErr:       true,
			expectedError: ErrVideoEmpty,
		},
		{
			name:          "Exceeds 50MB",
			filename:      "huge_footage.mp4",
			contentType:   "video/mp4",
			fileSize:      52 * 1024 * 1024,
			wantErr:       true,
			expectedError: ErrVideoTooLarge,
		},
		{
			name:          "Disallowed extension (MKV)",
			filename:      "video.mkv",
			contentType:   "video/x-matroska",
			fileSize:      10 * 1024 * 1024,
			wantErr:       true,
			expectedError: ErrInvalidVideoType,
		},
		{
			name:          "Disallowed extension (AVI)",
			filename:      "video.avi",
			contentType:   "video/x-msvideo",
			fileSize:      10 * 1024 * 1024,
			wantErr:       true,
			expectedError: ErrInvalidVideoType,
		},
		{
			name:          "Disallowed extension (EXE disguised)",
			filename:      "malicious.exe",
			contentType:   "video/mp4",
			fileSize:      1024,
			wantErr:       true,
			expectedError: ErrInvalidVideoType,
		},
		{
			name:         "Browser sent application/octet-stream fallback",
			filename:     "hero.mp4",
			contentType:  "application/octet-stream",
			fileSize:     5 * 1024 * 1024,
			wantErr:      false,
			expectedExt:  ".mp4",
			expectedMime: "video/mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, mime, err := ValidateHeroVideoUploadRequest(tt.filename, tt.contentType, tt.fileSize)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.expectedError != nil && err != tt.expectedError {
					t.Fatalf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if ext != tt.expectedExt {
					t.Errorf("expected extension %q, got %q", tt.expectedExt, ext)
				}
				if mime != tt.expectedMime {
					t.Errorf("expected mime %q, got %q", tt.expectedMime, mime)
				}
			}
		})
	}
}

func TestInspectAndValidateVideo(t *testing.T) {
	// Simulated valid MP4 header (bytes 4..8 = 'ftyp')
	mp4Header := make([]byte, 1024)
	copy(mp4Header[4:8], []byte("ftyp"))
	copy(mp4Header[8:12], []byte("mp42"))

	// Simulated valid WebM header (0x1A 0x45 0xDF 0xA3)
	webmHeader := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x00, 0x00, 0x1F, 0x42, 0x86, 0x81, 0x01}

	t.Run("Valid MP4 payload inspection", func(t *testing.T) {
		data, meta, err := InspectAndValidateVideo(bytes.NewReader(mp4Header), MaxHeroVideoSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Format != "mp4" {
			t.Errorf("expected format mp4, got %s", meta.Format)
		}
		if meta.ContentType != "video/mp4" {
			t.Errorf("expected content_type video/mp4, got %s", meta.ContentType)
		}
		if int64(len(data)) != 1024 {
			t.Errorf("expected 1024 bytes, got %d", len(data))
		}
	})

	t.Run("Valid WebM payload inspection", func(t *testing.T) {
		data, meta, err := InspectAndValidateVideo(bytes.NewReader(webmHeader), MaxHeroVideoSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Format != "webm" {
			t.Errorf("expected format webm, got %s", meta.Format)
		}
		if meta.ContentType != "video/webm" {
			t.Errorf("expected content_type video/webm, got %s", meta.ContentType)
		}
		if int64(len(data)) != int64(len(webmHeader)) {
			t.Errorf("expected %d bytes, got %d", len(webmHeader), len(data))
		}
	})

	t.Run("Reject non-video binary data", func(t *testing.T) {
		corrupt := []byte("PK\x03\x04This is a zip file disguised as video")
		_, _, err := InspectAndValidateVideo(bytes.NewReader(corrupt), MaxHeroVideoSize)
		if err == nil {
			t.Fatalf("expected error rejecting corrupt/non-video payload, got nil")
		}
	})

	t.Run("Reject too small payload", func(t *testing.T) {
		tiny := []byte("short")
		_, _, err := InspectAndValidateVideo(bytes.NewReader(tiny), MaxHeroVideoSize)
		if err == nil {
			t.Fatalf("expected error for tiny payload, got nil")
		}
	})
}

func TestHeroKeyGeneration(t *testing.T) {
	key := GenerateHeroVideoKey(".mp4")
	if !strings.HasPrefix(key, HeroVideoPrefix) {
		t.Errorf("expected key to start with %q, got %q", HeroVideoPrefix, key)
	}
	if !strings.HasSuffix(key, ".mp4") {
		t.Errorf("expected key to end with .mp4, got %q", key)
	}

	posterKey := GenerateHeroPosterKey(key, ".jpg")
	if !strings.HasPrefix(posterKey, HeroVideoPrefix) {
		t.Errorf("expected poster key to start with %q, got %q", HeroVideoPrefix, posterKey)
	}
	if !strings.HasSuffix(posterKey, "-poster.jpg") {
		t.Errorf("expected poster key to end with -poster.jpg, got %q", posterKey)
	}
}
