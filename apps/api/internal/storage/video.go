package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	MaxHeroVideoSize = 50 * 1024 * 1024 // 50 MB
	HeroVideoPrefix  = "website/hero-videos/"
)

var (
	ErrVideoTooLarge    = errors.New("video size exceeds maximum permitted limit of 50 MB")
	ErrVideoEmpty       = errors.New("video payload is empty")
	ErrInvalidVideoType = errors.New("unsupported video format; only MP4 (H.264) and WebM formats are supported")
	ErrCorruptVideo     = errors.New("video payload header is invalid or corrupted")
)

var allowedVideoContentTypes = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

var allowedVideoExtensions = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
}

type VideoMetadata struct {
	Format      string `json:"format"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ObjectKey   string `json:"object_key"`
	PublicURL   string `json:"public_url"`
}

// ValidateHeroVideoUploadRequest validates the metadata before generating a presigned upload URL.
func ValidateHeroVideoUploadRequest(filename, contentType string, fileSize int64) (string, string, error) {
	if fileSize <= 0 {
		return "", "", ErrVideoEmpty
	}
	if fileSize > MaxHeroVideoSize {
		return "", "", ErrVideoTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	expectedMime, extOk := allowedVideoExtensions[ext]
	if !extOk {
		return "", "", ErrInvalidVideoType
	}

	cleanContentType := strings.ToLower(strings.TrimSpace(contentType))
	if cleanContentType == "" || cleanContentType == "application/octet-stream" {
		cleanContentType = expectedMime
	}

	canonicalExt, mimeOk := allowedVideoContentTypes[cleanContentType]
	if !mimeOk {
		// If browser sent generic MIME, fall back to the extension-derived MIME
		if expectedMime != "" {
			cleanContentType = expectedMime
			canonicalExt = allowedVideoContentTypes[expectedMime]
		} else {
			return "", "", ErrInvalidVideoType
		}
	}

	return canonicalExt, cleanContentType, nil
}

// InspectAndValidateVideo inspects the first chunk of video stream to verify magic bytes and size constraints.
func InspectAndValidateVideo(r io.Reader, maxBytes int64) ([]byte, *VideoMetadata, error) {
	if maxBytes <= 0 || maxBytes > MaxHeroVideoSize {
		maxBytes = MaxHeroVideoSize
	}

	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read video payload: %w", err)
	}

	if len(data) == 0 {
		return nil, nil, ErrVideoEmpty
	}
	if int64(len(data)) > MaxHeroVideoSize {
		return nil, nil, ErrVideoTooLarge
	}

	// Validate magic bytes
	detectedFormat, detectedMime, err := detectVideoFormat(data)
	if err != nil {
		return nil, nil, err
	}

	meta := &VideoMetadata{
		Format:      detectedFormat,
		ContentType: detectedMime,
		Size:        int64(len(data)),
	}

	return data, meta, nil
}

// detectVideoFormat checks standard video file headers (MP4 / WebM).
func detectVideoFormat(data []byte) (string, string, error) {
	if len(data) < 12 {
		return "", "", ErrCorruptVideo
	}

	// MP4: usually has 'ftyp' at bytes 4..8
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		return "mp4", "video/mp4", nil
	}

	// WebM / Matroska: starts with EBML ID: 0x1A 0x45 0xDF 0xA3
	if len(data) >= 4 && bytes.Equal(data[0:4], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "webm", "video/webm", nil
	}

	return "", "", ErrInvalidVideoType
}

// GenerateHeroVideoKey constructs an isolated, unique server-controlled object key.
// Pattern: website/hero-videos/<uuid>.<ext>
func GenerateHeroVideoKey(ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "" {
		cleanExt = "mp4"
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s.%s", HeroVideoPrefix, uniqueID, cleanExt)
}

// GenerateHeroPosterKey constructs an isolated object key for an optional video poster.
// Pattern: website/hero-videos/<uuid>-poster.<ext>
func GenerateHeroPosterKey(videoKey, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "" {
		cleanExt = "jpg"
	}
	base := strings.TrimSuffix(videoKey, filepath.Ext(videoKey))
	return fmt.Sprintf("%s-poster.%s", base, cleanExt)
}
