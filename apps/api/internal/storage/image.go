package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	MaxProductImageSize    = 5 * 1024 * 1024 // 5 MB
	MaxImagesPerProduct    = 5
	MarketplaceImagePrefix      = "marketplace/products/"
	CommunityRequestImagePrefix = "community/requests/"
	CommunityProjectImagePrefix = "community/projects/"
)

var (
	ErrImageTooLarge       = errors.New("image size exceeds maximum permitted limit of 5 MB")
	ErrImageEmpty          = errors.New("image payload is empty")
	ErrInvalidImageType    = errors.New("unsupported image format; only JPG, JPEG, PNG, and WebP are allowed")
	ErrCorruptImage        = errors.New("image payload is malformed or corrupted")
	ErrInvalidDimensions   = errors.New("image dimensions are invalid or exceed allowable limits")
	ErrTooManyImages       = errors.New("a maximum of 5 images is permitted per product")
	ErrUnauthorizedKey     = errors.New("unauthorized image key; key does not belong to your business")
)

// Allowed MIME types and their canonical extensions
var allowedContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var allowedExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// ImageMetadata contains decoded image properties.
type ImageMetadata struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ObjectKey   string `json:"object_key"`
	PublicURL   string `json:"public_url"`
}

// ValidateUploadRequest validates the metadata before generating a presigned upload URL.
func ValidateUploadRequest(filename, contentType string, fileSize int64) (string, string, error) {
	if fileSize <= 0 {
		return "", "", ErrImageEmpty
	}
	if fileSize > MaxProductImageSize {
		return "", "", ErrImageTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	expectedMime, extOk := allowedExtensions[ext]
	if !extOk {
		return "", "", ErrInvalidImageType
	}

	cleanContentType := strings.ToLower(strings.TrimSpace(contentType))
	// Some browsers pass generic or alternative MIME types
	if cleanContentType == "" || cleanContentType == "application/octet-stream" {
		cleanContentType = expectedMime
	}

	canonicalExt, mimeOk := allowedContentTypes[cleanContentType]
	if !mimeOk {
		return "", "", ErrInvalidImageType
	}

	// Normalise extension (e.g. .jpeg -> .jpg)
	return canonicalExt, cleanContentType, nil
}

// InspectAndValidateImage decodes the image stream, verifies magic bytes, formats, and dimension sanity.
// Returns a buffer with the full image data ready for storage and the inspected metadata.
func InspectAndValidateImage(r io.Reader, maxBytes int64) ([]byte, *ImageMetadata, error) {
	if maxBytes <= 0 || maxBytes > MaxProductImageSize {
		maxBytes = MaxProductImageSize
	}

	// Read up to maxBytes + 1 to detect size violations
	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read image payload: %w", err)
	}

	if len(data) == 0 {
		return nil, nil, ErrImageEmpty
	}
	if int64(len(data)) > MaxProductImageSize {
		return nil, nil, ErrImageTooLarge
	}

	// 1. Detect MIME type via magic bytes
	detectedMime := http.DetectContentType(data)
	// Handle webp detection: http.DetectContentType might detect webp as "image/webp" or "application/octet-stream"
	if strings.HasPrefix(detectedMime, "image/webp") || (len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP") {
		detectedMime = "image/webp"
	}

	canonicalExt, mimeOk := allowedContentTypes[detectedMime]
	if !mimeOk {
		return nil, nil, fmt.Errorf("%w: detected content type %q is not permitted", ErrInvalidImageType, detectedMime)
	}

	// 2. Decode image header to verify actual structure and dimensions
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to decode image structure: %v", ErrCorruptImage, err)
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, nil, ErrInvalidDimensions
	}
	if cfg.Width > 12000 || cfg.Height > 12000 {
		return nil, nil, fmt.Errorf("%w: dimension %dx%d exceeds maximum allowed 12000x12000", ErrInvalidDimensions, cfg.Width, cfg.Height)
	}

	meta := &ImageMetadata{
		Width:       cfg.Width,
		Height:      cfg.Height,
		Format:      format,
		ContentType: detectedMime,
		Size:        int64(len(data)),
	}

	_ = canonicalExt
	return data, meta, nil
}

// GenerateProductImageKey constructs an isolated, unique server-controlled object key.
// Pattern: marketplace/products/<business_id>/<uuid>.<ext>
func GenerateProductImageKey(businessID, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "jpeg" {
		cleanExt = "jpg"
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s/%s.%s", MarketplaceImagePrefix, businessID, uniqueID, cleanExt)
}

// ValidateKeyBelongsToBusiness ensures that an image key belongs exclusively to the given business ID.
func ValidateKeyBelongsToBusiness(key, businessID string) bool {
	cleanKey := strings.TrimPrefix(key, "/")
	expectedPrefix := fmt.Sprintf("%s%s/", MarketplaceImagePrefix, businessID)
	return strings.HasPrefix(cleanKey, expectedPrefix)
}

// GenerateCommunityRequestImageKey constructs an isolated, unique server-controlled object key for request evidence.
// Pattern: community/requests/<request_id>/<uuid>.<ext>
func GenerateCommunityRequestImageKey(requestID, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "jpeg" {
		cleanExt = "jpg"
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s/%s.%s", CommunityRequestImagePrefix, requestID, uniqueID, cleanExt)
}

// GenerateCommunityProjectImageKey constructs an isolated, unique server-controlled object key for project evidence/milestones.
// Pattern: community/projects/<project_id>/<uuid>.<ext>
func GenerateCommunityProjectImageKey(projectID, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "jpeg" {
		cleanExt = "jpg"
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s/%s.%s", CommunityProjectImagePrefix, projectID, uniqueID, cleanExt)
}
