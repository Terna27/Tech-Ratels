package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	MaxEvidenceFileSize = 25 * 1024 * 1024 // 25 MB
	SafeguardingPrefix  = "safeguarding/cases/"
)

var (
	ErrEvidenceTooLarge       = errors.New("evidence file size exceeds maximum limit of 25 MB")
	ErrEvidenceEmpty          = errors.New("evidence payload is empty")
	ErrDisallowedFileType     = errors.New("disallowed file type; executables, scripts, and unsafe formats are strictly prohibited")
	ErrInvalidEvidenceKey     = errors.New("invalid or unauthorized safeguarding storage key")
	ErrSafeguardingUnconfig   = errors.New("private safeguarding storage is unconfigured")
)

var safeFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// Allowed MIME types for safeguarding evidence
var allowedEvidenceMIMEs = map[string]bool{
	"image/jpeg":       true,
	"image/jpg":        true,
	"image/png":        true,
	"image/webp":       true,
	"application/pdf":  true,
	"text/plain":       true,
	"audio/mpeg":       true,
	"audio/mp4":        true,
	"audio/wav":        true,
	"audio/x-wav":      true,
	"video/mp4":        true,
	"video/webm":       true,
	"video/quicktime":  true,
}

var disallowedExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".sh": true, ".bin": true,
	".js": true, ".html": true, ".htm": true, ".php": true, ".py": true,
	".vbs": true, ".scr": true, ".com": true, ".dll": true, ".so": true,
}

// SafeguardingStorageService defines the contract for strictly private evidence storage.
type SafeguardingStorageService interface {
	IsConfigured() bool
	StoreEvidence(ctx context.Context, caseID, originalFileName, contentType string, reader io.Reader, size int64) (storageKey string, sha256Hex string, actualSize int64, err error)
	GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error)
	ReadEvidence(ctx context.Context, storageKey string) ([]byte, error)
	DeleteEvidence(ctx context.Context, storageKey string) error
}

// R2SafeguardingStorage implements private evidence storage on Cloudflare R2 without public URLs.
type R2SafeguardingStorage struct {
	bucketName    string
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

func NewR2SafeguardingStorage(client *s3.Client, presignClient *s3.PresignClient, bucketName string) *R2SafeguardingStorage {
	return &R2SafeguardingStorage{
		bucketName:    bucketName,
		s3Client:      client,
		presignClient: presignClient,
	}
}

func (s *R2SafeguardingStorage) IsConfigured() bool {
	return s != nil && s.s3Client != nil && s.bucketName != ""
}

func (s *R2SafeguardingStorage) StoreEvidence(ctx context.Context, caseID, originalFileName, contentType string, reader io.Reader, size int64) (string, string, int64, error) {
	if !s.IsConfigured() {
		return "", "", 0, ErrSafeguardingUnconfig
	}

	sanitizedName, mime, err := ValidateEvidenceFile(originalFileName, contentType, size)
	if err != nil {
		return "", "", 0, err
	}

	evidenceID := uuid.New().String()
	storageKey := fmt.Sprintf("%s%s/evidence/%s_%s", SafeguardingPrefix, caseID, evidenceID, sanitizedName)

	hasher := sha256.New()
	teeReader := io.TeeReader(reader, hasher)

	buf, err := io.ReadAll(teeReader)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read evidence payload: %w", err)
	}

	actualSize := int64(len(buf))
	if actualSize == 0 {
		return "", "", 0, ErrEvidenceEmpty
	}
	if actualSize > MaxEvidenceFileSize {
		return "", "", 0, ErrEvidenceTooLarge
	}

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(storageKey),
		Body:        strings.NewReader(string(buf)),
		ContentType: aws.String(mime),
		Metadata: map[string]string{
			"safeguarding-case-id": caseID,
			"sha256":               sha256Hex,
			"original-filename":    sanitizedName,
		},
	})
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to upload evidence to private storage: %w", err)
	}

	return storageKey, sha256Hex, actualSize, nil
}

func (s *R2SafeguardingStorage) GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error) {
	if !s.IsConfigured() {
		return "", ErrSafeguardingUnconfig
	}
	if !strings.HasPrefix(storageKey, SafeguardingPrefix) {
		return "", ErrInvalidEvidenceKey
	}

	if expiry <= 0 || expiry > 15*time.Minute {
		expiry = 5 * time.Minute
	}

	safeName := safeFilenameRegex.ReplaceAllString(filepath.Base(downloadFileName), "_")
	disposition := fmt.Sprintf(`attachment; filename="%s"`, safeName)

	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucketName),
		Key:                        aws.String(storageKey),
		ResponseContentDisposition: aws.String(disposition),
		ResponseCacheControl:       aws.String("private, no-store, max-age=0"),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}

	return req.URL, nil
}

func (s *R2SafeguardingStorage) ReadEvidence(ctx context.Context, storageKey string) ([]byte, error) {
	if !s.IsConfigured() {
		return nil, ErrSafeguardingUnconfig
	}
	if !strings.HasPrefix(storageKey, SafeguardingPrefix) {
		return nil, ErrInvalidEvidenceKey
	}

	out, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read evidence object: %w", err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

func (s *R2SafeguardingStorage) DeleteEvidence(ctx context.Context, storageKey string) error {
	if !s.IsConfigured() {
		return ErrSafeguardingUnconfig
	}
	if !strings.HasPrefix(storageKey, SafeguardingPrefix) {
		return ErrInvalidEvidenceKey
	}

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storageKey),
	})
	return err
}

// DevSafeguardingStorageService is an in-memory mock for local development and testing.
type DevSafeguardingStorageService struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewDevSafeguardingStorageService() *DevSafeguardingStorageService {
	return &DevSafeguardingStorageService{
		files: make(map[string][]byte),
	}
}

func (d *DevSafeguardingStorageService) IsConfigured() bool {
	return true
}

func (d *DevSafeguardingStorageService) StoreEvidence(ctx context.Context, caseID, originalFileName, contentType string, reader io.Reader, size int64) (string, string, int64, error) {
	sanitizedName, _, err := ValidateEvidenceFile(originalFileName, contentType, size)
	if err != nil {
		return "", "", 0, err
	}

	hasher := sha256.New()
	teeReader := io.TeeReader(reader, hasher)

	buf, err := io.ReadAll(teeReader)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read evidence payload: %w", err)
	}

	actualSize := int64(len(buf))
	if actualSize == 0 {
		return "", "", 0, ErrEvidenceEmpty
	}
	if actualSize > MaxEvidenceFileSize {
		return "", "", 0, ErrEvidenceTooLarge
	}

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))
	evidenceID := uuid.New().String()
	storageKey := fmt.Sprintf("%s%s/evidence/%s_%s", SafeguardingPrefix, caseID, evidenceID, sanitizedName)

	d.mu.Lock()
	d.files[storageKey] = buf
	d.mu.Unlock()

	return storageKey, sha256Hex, actualSize, nil
}

func (d *DevSafeguardingStorageService) GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error) {
	if !strings.HasPrefix(storageKey, SafeguardingPrefix) {
		return "", ErrInvalidEvidenceKey
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if _, ok := d.files[storageKey]; !ok {
		return "", ErrObjectNotFound
	}
	// Return local pseudo-download URL for sandbox verification
	safeName := safeFilenameRegex.ReplaceAllString(filepath.Base(downloadFileName), "_")
	return fmt.Sprintf("/api/v1/admin/safeguarding/evidence/sandbox-download?key=%s&filename=%s", storageKey, safeName), nil
}

func (d *DevSafeguardingStorageService) ReadEvidence(ctx context.Context, storageKey string) ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	data, ok := d.files[storageKey]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return data, nil
}

func (d *DevSafeguardingStorageService) DeleteEvidence(ctx context.Context, storageKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.files, storageKey)
	return nil
}

// ValidateEvidenceFile checks filename extensions, detected content type, and payload size.
func ValidateEvidenceFile(filename, declaredMIME string, declaredSize int64) (sanitizedName string, validatedMIME string, err error) {
	if declaredSize > MaxEvidenceFileSize {
		return "", "", ErrEvidenceTooLarge
	}

	cleanBase := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(cleanBase))
	if ext == "" || disallowedExtensions[ext] {
		return "", "", ErrDisallowedFileType
	}

	sanitizedName = safeFilenameRegex.ReplaceAllString(cleanBase, "_")

	mime := strings.ToLower(strings.TrimSpace(declaredMIME))
	if idx := strings.Index(mime, ";"); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}

	if !allowedEvidenceMIMEs[mime] {
		// Attempt fallback by extension
		switch ext {
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".png":
			mime = "image/png"
		case ".webp":
			mime = "image/webp"
		case ".pdf":
			mime = "application/pdf"
		case ".txt":
			mime = "text/plain"
		case ".mp4":
			mime = "video/mp4"
		case ".webm":
			mime = "video/webm"
		case ".mp3":
			mime = "audio/mpeg"
		default:
			return "", "", ErrDisallowedFileType
		}
	}

	return sanitizedName, mime, nil
}
