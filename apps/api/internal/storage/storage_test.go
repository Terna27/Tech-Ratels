package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"time"
)

func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestValidateUploadRequest(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		fileSize    int64
		wantErr     error
		wantExt     string
	}{
		{
			name:        "Valid JPG",
			filename:    "photo.jpg",
			contentType: "image/jpeg",
			fileSize:    1024 * 100,
			wantErr:     nil,
			wantExt:     ".jpg",
		},
		{
			name:        "Valid PNG",
			filename:    "logo.PNG",
			contentType: "image/png",
			fileSize:    1024 * 500,
			wantErr:     nil,
			wantExt:     ".png",
		},
		{
			name:        "Valid WebP",
			filename:    "banner.webp",
			contentType: "image/webp",
			fileSize:    1024 * 300,
			wantErr:     nil,
			wantExt:     ".webp",
		},
		{
			name:        "Empty file",
			filename:    "empty.jpg",
			contentType: "image/jpeg",
			fileSize:    0,
			wantErr:     ErrImageEmpty,
		},
		{
			name:        "Exceeds 5MB",
			filename:    "huge.jpg",
			contentType: "image/jpeg",
			fileSize:    MaxProductImageSize + 1,
			wantErr:     ErrImageTooLarge,
		},
		{
			name:        "Disallowed extension (PDF)",
			filename:    "document.pdf",
			contentType: "application/pdf",
			fileSize:    1024,
			wantErr:     ErrInvalidImageType,
		},
		{
			name:        "Disallowed extension (SVG)",
			filename:    "vector.svg",
			contentType: "image/svg+xml",
			fileSize:    1024,
			wantErr:     ErrInvalidImageType,
		},
		{
			name:        "Disallowed extension (EXE disguised)",
			filename:    "app.exe",
			contentType: "application/x-msdownload",
			fileSize:    1024,
			wantErr:     ErrInvalidImageType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, _, err := ValidateUploadRequest(tt.filename, tt.contentType, tt.fileSize)
			if tt.wantErr != nil {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if ext != tt.wantExt {
					t.Fatalf("expected ext %s, got %s", tt.wantExt, ext)
				}
			}
		})
	}
}

func TestInspectAndValidateImage(t *testing.T) {
	t.Run("Valid JPEG inspection", func(t *testing.T) {
		data := createTestJPEG(200, 150)
		buf, meta, err := InspectAndValidateImage(bytes.NewReader(data), MaxProductImageSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(buf) != len(data) {
			t.Fatalf("expected buffer length %d, got %d", len(data), len(buf))
		}
		if meta.Width != 200 || meta.Height != 150 {
			t.Fatalf("expected 200x150, got %dx%d", meta.Width, meta.Height)
		}
		if meta.ContentType != "image/jpeg" {
			t.Fatalf("expected image/jpeg, got %s", meta.ContentType)
		}
	})

	t.Run("Valid PNG inspection", func(t *testing.T) {
		data := createTestPNG(300, 300)
		_, meta, err := InspectAndValidateImage(bytes.NewReader(data), MaxProductImageSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Width != 300 || meta.Height != 300 {
			t.Fatalf("expected 300x300, got %dx%d", meta.Width, meta.Height)
		}
		if meta.ContentType != "image/png" {
			t.Fatalf("expected image/png, got %s", meta.ContentType)
		}
	})

	t.Run("Reject non-image disguised as image", func(t *testing.T) {
		fakeImage := []byte("<html><body>This is an HTML file not an image</body></html>")
		_, _, err := InspectAndValidateImage(bytes.NewReader(fakeImage), MaxProductImageSize)
		if err == nil {
			t.Fatal("expected error for fake image, got nil")
		}
		if !errors.Is(err, ErrInvalidImageType) {
			t.Fatalf("expected ErrInvalidImageType, got %v", err)
		}
	})

	t.Run("Reject corrupt image payload", func(t *testing.T) {
		// JPEG magic bytes followed by garbage
		corruptJPEG := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xAA, 0xBB, 0xCC}
		_, _, err := InspectAndValidateImage(bytes.NewReader(corruptJPEG), MaxProductImageSize)
		if err == nil {
			t.Fatal("expected error for corrupt image, got nil")
		}
		if !errors.Is(err, ErrCorruptImage) {
			t.Fatalf("expected ErrCorruptImage, got %v", err)
		}
	})
}

func TestKeyGenerationAndBusinessIsolation(t *testing.T) {
	bizA := "biz_11111111-1111-1111-1111-111111111111"
	bizB := "biz_22222222-2222-2222-2222-222222222222"

	keyA := GenerateProductImageKey(bizA, ".jpg")
	if !strings.HasPrefix(keyA, "marketplace/products/"+bizA+"/") {
		t.Fatalf("key %s does not have expected prefix", keyA)
	}
	if !strings.HasSuffix(keyA, ".jpg") {
		t.Fatalf("key %s does not end with .jpg", keyA)
	}

	if !ValidateKeyBelongsToBusiness(keyA, bizA) {
		t.Fatalf("key %s should belong to bizA", keyA)
	}
	if ValidateKeyBelongsToBusiness(keyA, bizB) {
		t.Fatalf("key %s should NOT belong to bizB", keyA)
	}
}

func TestDevSandboxStorageService(t *testing.T) {
	svc := NewDevSandboxStorageService("https://images.ratels.org.ng")
	ctx := context.Background()

	if !svc.IsConfigured() {
		t.Fatal("DevSandbox should be configured")
	}

	key := "marketplace/products/biz1/photo.jpg"
	presigned, err := svc.GeneratePresignedUpload(ctx, key, "image/jpeg", 50000, 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating presigned: %v", err)
	}
	if !strings.Contains(presigned, key) {
		t.Fatalf("presigned url %s should contain key", presigned)
	}

	// Upload object
	content := []byte("image_data_here")
	pubURL, err := svc.UploadObject(ctx, key, "image/jpeg", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("unexpected error uploading: %v", err)
	}
	expectedPub := "https://images.ratels.org.ng/" + key
	if pubURL != expectedPub {
		t.Fatalf("expected public url %s, got %s", expectedPub, pubURL)
	}

	// Retrieve object
	retrieved, ct, found := svc.GetObject(key)
	if !found || string(retrieved) != "image_data_here" || ct != "image/jpeg" {
		t.Fatalf("failed retrieving object")
	}

	// Delete object
	if err := svc.DeleteObject(ctx, key); err != nil {
		t.Fatalf("failed deleting: %v", err)
	}
	_, _, found = svc.GetObject(key)
	if found {
		t.Fatal("object should be deleted")
	}
}

func TestUnconfiguredStorageService(t *testing.T) {
	svc := NewUnconfiguredStorageService()
	ctx := context.Background()

	if svc.IsConfigured() {
		t.Fatal("UnconfiguredStorageService should report false for IsConfigured")
	}

	_, err := svc.GeneratePresignedUpload(ctx, "key", "image/jpeg", 100, time.Minute)
	if !errors.Is(err, ErrStorageUnconfigured) {
		t.Fatalf("expected ErrStorageUnconfigured, got %v", err)
	}

	_, err = svc.UploadObject(ctx, "key", "image/jpeg", strings.NewReader("abc"), 3)
	if !errors.Is(err, ErrStorageUnconfigured) {
		t.Fatalf("expected ErrStorageUnconfigured, got %v", err)
	}

	err = svc.DeleteObject(ctx, "key")
	if !errors.Is(err, ErrStorageUnconfigured) {
		t.Fatalf("expected ErrStorageUnconfigured, got %v", err)
	}
}
