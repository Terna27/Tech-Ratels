package storage

import (
	"bytes"
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
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	MaxExportFileSize   = 50 * 1024 * 1024 // 50 MB
	ReportExportPrefix  = "reports/exports/private/"
)

var (
	ErrExportTooLarge      = errors.New("export file size exceeds maximum limit of 50 MB")
	ErrExportEmpty         = errors.New("export payload is empty")
	ErrInvalidExportKey    = errors.New("invalid or unauthorized export storage key")
	ErrReportExportUnconfig = errors.New("private report export storage is unconfigured")
)

var safeExportFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// ReportExportStorage defines the contract for strictly private report export storage on Cloudflare R2 or Dev Sandbox.
type ReportExportStorage interface {
	IsConfigured() bool
	StoreExport(ctx context.Context, snapshotID string, format string, originalFileName string, contentType string, data []byte) (storageKey string, sha256Hex string, size int64, err error)
	GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error)
	ReadExport(ctx context.Context, storageKey string) ([]byte, error)
	DeleteExport(ctx context.Context, storageKey string) error
}

// R2ReportExportStorage implements private export storage on Cloudflare R2 without public URLs.
type R2ReportExportStorage struct {
	bucketName    string
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

func NewR2ReportExportStorage(client *s3.Client, presignClient *s3.PresignClient, bucketName string) *R2ReportExportStorage {
	return &R2ReportExportStorage{
		bucketName:    bucketName,
		s3Client:      client,
		presignClient: presignClient,
	}
}

// NewR2ReportExportStorageFromConfig creates a private Cloudflare R2 storage client specifically
// for report exports from the provided R2Config.
func NewR2ReportExportStorageFromConfig(ctx context.Context, cfg R2Config) (*R2ReportExportStorage, error) {
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.BucketName == "" {
		return nil, errors.New("R2 access key, secret key, and bucket name are required")
	}

	endpointURL := cfg.Endpoint
	if endpointURL == "" {
		if cfg.AccountID == "" {
			return nil, errors.New("R2 Account ID or custom Endpoint is required")
		}
		endpointURL = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           endpointURL,
			SigningRegion: "auto",
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS/R2 configuration for report exports: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	presignClient := s3.NewPresignClient(s3Client)

	return NewR2ReportExportStorage(s3Client, presignClient, cfg.BucketName), nil
}

func (s *R2ReportExportStorage) IsConfigured() bool {
	return s != nil && s.s3Client != nil && s.bucketName != ""
}

func (s *R2ReportExportStorage) StoreExport(ctx context.Context, snapshotID string, format string, originalFileName string, contentType string, data []byte) (string, string, int64, error) {
	if !s.IsConfigured() {
		return "", "", 0, ErrReportExportUnconfig
	}

	actualSize := int64(len(data))
	if actualSize == 0 {
		return "", "", 0, ErrExportEmpty
	}
	if actualSize > MaxExportFileSize {
		return "", "", 0, ErrExportTooLarge
	}

	sanitizedName := safeExportFilenameRegex.ReplaceAllString(filepath.Base(originalFileName), "_")
	if sanitizedName == "" {
		sanitizedName = fmt.Sprintf("report_%s.%s", snapshotID, format)
	}

	exportID := uuid.New().String()
	storageKey := fmt.Sprintf("%s%s/%s/%s_%s", ReportExportPrefix, snapshotID, format, exportID, sanitizedName)

	hasher := sha256.New()
	hasher.Write(data)
	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(storageKey),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(actualSize),
		Metadata: map[string]string{
			"snapshot-id": snapshotID,
			"format":      format,
			"sha256":      sha256Hex,
			"export-id":   exportID,
		},
	})
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to upload report export to private storage: %w", err)
	}

	return storageKey, sha256Hex, actualSize, nil
}

func (s *R2ReportExportStorage) GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error) {
	if !s.IsConfigured() {
		return "", ErrReportExportUnconfig
	}
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return "", ErrInvalidExportKey
	}

	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	safeDownloadName := safeExportFilenameRegex.ReplaceAllString(filepath.Base(downloadFileName), "_")
	disposition := fmt.Sprintf("attachment; filename=\"%s\"", safeDownloadName)

	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucketName),
		Key:                        aws.String(storageKey),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}

	return req.URL, nil
}

func (s *R2ReportExportStorage) ReadExport(ctx context.Context, storageKey string) ([]byte, error) {
	if !s.IsConfigured() {
		return nil, ErrReportExportUnconfig
	}
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return nil, ErrInvalidExportKey
	}

	out, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read export from private storage: %w", err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

func (s *R2ReportExportStorage) DeleteExport(ctx context.Context, storageKey string) error {
	if !s.IsConfigured() {
		return ErrReportExportUnconfig
	}
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return ErrInvalidExportKey
	}

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete export from private storage: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------
// DevSandboxReportExportStorage (In-memory mock for dev & testing)
// -----------------------------------------------------------------------

type storedExportObject struct {
	data        []byte
	contentType string
	sha256Hex   string
	uploadedAt  time.Time
}

type DevSandboxReportExportStorage struct {
	mu      sync.RWMutex
	objects map[string]storedExportObject
}

func NewDevSandboxReportExportStorage() *DevSandboxReportExportStorage {
	return &DevSandboxReportExportStorage{
		objects: make(map[string]storedExportObject),
	}
}

func (s *DevSandboxReportExportStorage) IsConfigured() bool {
	return true
}

func (s *DevSandboxReportExportStorage) StoreExport(ctx context.Context, snapshotID string, format string, originalFileName string, contentType string, data []byte) (string, string, int64, error) {
	actualSize := int64(len(data))
	if actualSize == 0 {
		return "", "", 0, ErrExportEmpty
	}
	if actualSize > MaxExportFileSize {
		return "", "", 0, ErrExportTooLarge
	}

	sanitizedName := safeExportFilenameRegex.ReplaceAllString(filepath.Base(originalFileName), "_")
	if sanitizedName == "" {
		sanitizedName = fmt.Sprintf("report_%s.%s", snapshotID, format)
	}

	exportID := uuid.New().String()
	storageKey := fmt.Sprintf("%s%s/%s/%s_%s", ReportExportPrefix, snapshotID, format, exportID, sanitizedName)

	hasher := sha256.New()
	hasher.Write(data)
	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.objects[storageKey] = storedExportObject{
		data:        data,
		contentType: contentType,
		sha256Hex:   sha256Hex,
		uploadedAt:  time.Now(),
	}

	return storageKey, sha256Hex, actualSize, nil
}

func (s *DevSandboxReportExportStorage) GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error) {
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return "", ErrInvalidExportKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.objects[storageKey]; !exists {
		return "", ErrObjectNotFound
	}

	safeName := safeExportFilenameRegex.ReplaceAllString(filepath.Base(downloadFileName), "_")
	return fmt.Sprintf("https://mock-r2-private.ratels.internal/download?key=%s&filename=%s&expires=%d", storageKey, safeName, time.Now().Add(expiry).Unix()), nil
}

func (s *DevSandboxReportExportStorage) ReadExport(ctx context.Context, storageKey string) ([]byte, error) {
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return nil, ErrInvalidExportKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, exists := s.objects[storageKey]
	if !exists {
		return nil, ErrObjectNotFound
	}

	result := make([]byte, len(obj.data))
	copy(result, obj.data)
	return result, nil
}

func (s *DevSandboxReportExportStorage) DeleteExport(ctx context.Context, storageKey string) error {
	if !strings.HasPrefix(storageKey, ReportExportPrefix) || strings.Contains(storageKey, "..") {
		return ErrInvalidExportKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, storageKey)
	return nil
}

// -----------------------------------------------------------------------
// UnconfiguredReportExportStorage (Production Safety Fallback)
// -----------------------------------------------------------------------

// UnconfiguredReportExportStorage implements ReportExportStorage as an explicit unconfigured fallback.
type UnconfiguredReportExportStorage struct{}

func NewUnconfiguredReportExportStorage() *UnconfiguredReportExportStorage {
	return &UnconfiguredReportExportStorage{}
}

func (s *UnconfiguredReportExportStorage) IsConfigured() bool {
	return false
}

func (s *UnconfiguredReportExportStorage) StoreExport(ctx context.Context, snapshotID string, format string, originalFileName string, contentType string, data []byte) (string, string, int64, error) {
	return "", "", 0, ErrReportExportUnconfig
}

func (s *UnconfiguredReportExportStorage) GeneratePresignedDownload(ctx context.Context, storageKey, downloadFileName string, expiry time.Duration) (string, error) {
	return "", ErrReportExportUnconfig
}

func (s *UnconfiguredReportExportStorage) ReadExport(ctx context.Context, storageKey string) ([]byte, error) {
	return nil, ErrReportExportUnconfig
}

func (s *UnconfiguredReportExportStorage) DeleteExport(ctx context.Context, storageKey string) error {
	return ErrReportExportUnconfig
}

