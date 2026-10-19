package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	MaxGalleryImageSize     = 10 * 1024 * 1024  // 10 MB
	MaxGalleryVideoSize     = 100 * 1024 * 1024 // 100 MB
	GalleryQuarantinePrefix = "gallery/quarantine/"
	GalleryEventsPrefix     = "gallery/events/"
	GalleryThumbnailsPrefix = "gallery/thumbnails/"
)

var (
	ErrGalleryImageTooLarge      = errors.New("gallery image exceeds maximum permitted size of 10 MB")
	ErrGalleryVideoTooLarge      = errors.New("gallery video exceeds maximum permitted size of 100 MB")
	ErrGalleryMediaEmpty         = errors.New("media payload is empty")
	ErrInvalidGalleryMediaType   = errors.New("unsupported media type; only 'image' and 'video' are permitted")
	ErrInvalidGalleryImageFormat = errors.New("unsupported image format; allowed formats: JPG, JPEG, PNG, WebP")
	ErrInvalidGalleryVideoFormat = errors.New("unsupported video format; allowed formats: MP4 (H.264) and WebM")
	ErrCorruptGalleryMedia       = errors.New("media payload header is malformed or corrupted")
)

var allowedGalleryImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var allowedGalleryImageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

var allowedGalleryVideoTypes = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

var allowedGalleryVideoExts = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
}

// GenerateGalleryPendingKey creates a private quarantine object key.
func GenerateGalleryPendingKey(mediaType, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "" {
		if mediaType == "video" {
			cleanExt = "mp4"
		} else {
			cleanExt = "jpg"
		}
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s/%s.%s", GalleryQuarantinePrefix, mediaType, uniqueID, cleanExt)
}

// GenerateGalleryPublicKey creates a sanitized public object key under the event path.
func GenerateGalleryPublicKey(eventID, mediaType, ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	if cleanExt == "" {
		if mediaType == "video" {
			cleanExt = "mp4"
		} else {
			cleanExt = "jpg"
		}
	}
	folder := "images"
	if mediaType == "video" {
		folder = "videos"
	}
	uniqueID := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s/%s/%s.%s", GalleryEventsPrefix, eventID, folder, uniqueID, cleanExt)
}

// GenerateGalleryThumbnailKey creates a thumbnail key for public delivery.
func GenerateGalleryThumbnailKey(mediaID string) string {
	cleanID := strings.ReplaceAll(mediaID, "-", "")
	return fmt.Sprintf("%s%s-thumb.jpg", GalleryThumbnailsPrefix, cleanID)
}

// ValidateGalleryUploadRequest checks metadata before upload initiation.
func ValidateGalleryUploadRequest(filename, contentType string, fileSize int64, mediaType string) (canonicalExt string, cleanMime string, err error) {
	if fileSize <= 0 {
		return "", "", ErrGalleryMediaEmpty
	}

	ext := strings.ToLower(filepath.Ext(filename))

	switch mediaType {
	case "image":
		if fileSize > MaxGalleryImageSize {
			return "", "", ErrGalleryImageTooLarge
		}
		expectedMime, extOk := allowedGalleryImageExts[ext]
		if !extOk {
			return "", "", ErrInvalidGalleryImageFormat
		}
		cleanContentType := strings.ToLower(strings.TrimSpace(contentType))
		if cleanContentType == "" || cleanContentType == "application/octet-stream" {
			cleanContentType = expectedMime
		}
		canonicalExt, mimeOk := allowedGalleryImageTypes[cleanContentType]
		if !mimeOk {
			if expectedMime != "" {
				cleanContentType = expectedMime
				canonicalExt = allowedGalleryImageTypes[expectedMime]
			} else {
				return "", "", ErrInvalidGalleryImageFormat
			}
		}
		return canonicalExt, cleanContentType, nil

	case "video":
		if fileSize > MaxGalleryVideoSize {
			return "", "", ErrGalleryVideoTooLarge
		}
		expectedMime, extOk := allowedGalleryVideoExts[ext]
		if !extOk {
			return "", "", ErrInvalidGalleryVideoFormat
		}
		cleanContentType := strings.ToLower(strings.TrimSpace(contentType))
		if cleanContentType == "" || cleanContentType == "application/octet-stream" {
			cleanContentType = expectedMime
		}
		canonicalExt, mimeOk := allowedGalleryVideoTypes[cleanContentType]
		if !mimeOk {
			if expectedMime != "" {
				cleanContentType = expectedMime
				canonicalExt = allowedGalleryVideoTypes[expectedMime]
			} else {
				return "", "", ErrInvalidGalleryVideoFormat
			}
		}
		return canonicalExt, cleanContentType, nil

	default:
		return "", "", ErrInvalidGalleryMediaType
	}
}

// InspectAndValidateGalleryUpload inspects image or video payload bytes, validates structure,
// and extracts basic metadata.
func InspectAndValidateGalleryUpload(r io.Reader, size int64, mediaType string) ([]byte, string, int, int, error) {
	maxBytes := MaxGalleryImageSize
	if mediaType == "video" {
		maxBytes = MaxGalleryVideoSize
	}
	if size > int64(maxBytes) {
		if mediaType == "video" {
			return nil, "", 0, 0, ErrGalleryVideoTooLarge
		}
		return nil, "", 0, 0, ErrGalleryImageTooLarge
	}

	lr := io.LimitReader(r, int64(maxBytes)+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("failed to read media stream: %w", err)
	}
	if len(data) == 0 {
		return nil, "", 0, 0, ErrGalleryMediaEmpty
	}
	if len(data) > maxBytes {
		if mediaType == "video" {
			return nil, "", 0, 0, ErrGalleryVideoTooLarge
		}
		return nil, "", 0, 0, ErrGalleryImageTooLarge
	}

	if mediaType == "image" {
		detectedMime := http.DetectContentType(data)
		if strings.HasPrefix(detectedMime, "image/webp") || (len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP") {
			detectedMime = "image/webp"
		}
		if _, ok := allowedGalleryImageTypes[detectedMime]; !ok {
			return nil, "", 0, 0, fmt.Errorf("%w: detected %s", ErrInvalidGalleryImageFormat, detectedMime)
		}

		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, "", 0, 0, fmt.Errorf("%w: %v", ErrCorruptGalleryMedia, err)
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return nil, "", 0, 0, ErrInvalidDimensions
		}

		return data, detectedMime, cfg.Width, cfg.Height, nil
	}

	if mediaType == "video" {
		format, mime, err := detectVideoFormat(data)
		if err != nil {
			return nil, "", 0, 0, err
		}
		_ = format
		return data, mime, 0, 0, nil
	}

	return nil, "", 0, 0, ErrInvalidGalleryMediaType
}

// SanitizeAndReencodeImage strips all EXIF, GPS, camera model, and device identifiers
// by decoding pure raster pixels and re-encoding with Go's standard clean JPEG encoder.
// Also generates an optimized thumbnail derivative.
func SanitizeAndReencodeImage(data []byte) (sanitizedBytes []byte, thumbBytes []byte, width int, height int, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to decode image for sanitization: %w", err)
	}

	bounds := img.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()

	// 1. Re-encode full image as clean JPEG (strips all EXIF/GPS tags entirely)
	var mainBuf bytes.Buffer
	if err := jpeg.Encode(&mainBuf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to re-encode sanitized image: %w", err)
	}
	sanitizedBytes = mainBuf.Bytes()

	// 2. Generate thumbnail (max 480x480 bounding box)
	thumbImg := resizeImageSimple(img, 480, 480)
	var thumbBuf bytes.Buffer
	if err := jpeg.Encode(&thumbBuf, thumbImg, &jpeg.Options{Quality: 80}); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to encode thumbnail: %w", err)
	}
	thumbBytes = thumbBuf.Bytes()

	return sanitizedBytes, thumbBytes, width, height, nil
}

// resizeImageSimple performs aspect-ratio preserving downsampling.
func resizeImageSimple(src image.Image, maxW, maxH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= maxW && srcH <= maxH {
		return src
	}

	// Calculate aspect ratio scale
	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	targetW := int(float64(srcW) * scale)
	targetH := int(float64(srcH) * scale)
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		srcY := bounds.Min.Y + int(float64(y)*float64(srcH)/float64(targetH))
		for x := 0; x < targetW; x++ {
			srcX := bounds.Min.X + int(float64(x)*float64(srcW)/float64(targetW))
			c := src.At(srcX, srcY)
			r, g, b, a := c.RGBA()
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}

	return dst
}
