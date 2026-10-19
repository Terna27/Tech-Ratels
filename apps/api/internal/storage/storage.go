package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrStorageUnconfigured = errors.New("cloud storage is not configured")
	ErrInvalidObjectKey    = errors.New("invalid or unauthorized object key")
	ErrObjectNotFound      = errors.New("storage object not found")
)

// StorageService defines the contract for cloud object storage (Cloudflare R2, S3-compatible, or dev mock).
type StorageService interface {
	IsConfigured() bool
	ProviderName() string
	GeneratePresignedUpload(ctx context.Context, key string, contentType string, maxBytes int64, expiry time.Duration) (string, error)
	UploadObject(ctx context.Context, key string, contentType string, reader io.Reader, size int64) (string, error)
	ReadObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
	GetPublicURL(key string) string
}

// R2Config holds credentials and endpoint parameters for Cloudflare R2.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	PublicDomain    string
	Endpoint        string // Optional custom endpoint override
}

// R2StorageService implements StorageService using Cloudflare R2 through AWS SDK S3 client.
type R2StorageService struct {
	bucketName    string
	publicDomain  string
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

// NewR2StorageService creates a new R2-backed storage service.
func NewR2StorageService(ctx context.Context, cfg R2Config) (*R2StorageService, error) {
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
		return nil, fmt.Errorf("failed to load AWS/R2 configuration: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(s3Client)

	pubDomain := strings.TrimRight(cfg.PublicDomain, "/")
	if pubDomain == "" {
		pubDomain = "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev"
	} else if !strings.HasPrefix(pubDomain, "http://") && !strings.HasPrefix(pubDomain, "https://") {
		pubDomain = "https://" + pubDomain
	}

	return &R2StorageService{
		bucketName:    cfg.BucketName,
		publicDomain:  pubDomain,
		s3Client:      s3Client,
		presignClient: presignClient,
	}, nil
}

func (s *R2StorageService) IsConfigured() bool {
	return true
}

func (s *R2StorageService) ProviderName() string {
	return "cloudflare_r2"
}

func (s *R2StorageService) GeneratePresignedUpload(ctx context.Context, key string, contentType string, maxBytes int64, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	putInput := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(maxBytes),
	}

	req, err := s.presignClient.PresignPutObject(ctx, putInput, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate R2 presigned PUT url: %w", err)
	}

	return req.URL, nil
}

func (s *R2StorageService) UploadObject(ctx context.Context, key string, contentType string, reader io.Reader, size int64) (string, error) {
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object to R2: %w", err)
	}

	return s.GetPublicURL(key), nil
}

func (s *R2StorageService) ReadObject(ctx context.Context, key string) ([]byte, error) {
	out, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from R2: %w", err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *R2StorageService) DeleteObject(ctx context.Context, key string) error {
	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from R2: %w", err)
	}
	return nil
}

func (s *R2StorageService) GetPublicURL(key string) string {
	cleanKey := strings.TrimPrefix(key, "/")
	return fmt.Sprintf("%s/%s", s.publicDomain, cleanKey)
}

// -------------------------------------------------------------
// DevSandboxStorageService (In-memory mock for dev & automated testing)
// -------------------------------------------------------------

type storedObject struct {
	data        []byte
	contentType string
	uploadedAt  time.Time
}

type DevSandboxStorageService struct {
	mu           sync.RWMutex
	objects      map[string]storedObject
	publicDomain string
}

func NewDevSandboxStorageService(baseURL string) *DevSandboxStorageService {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://images.ratels.org.ng"
	}
	return &DevSandboxStorageService{
		objects:      make(map[string]storedObject),
		publicDomain: base,
	}
}

func (s *DevSandboxStorageService) IsConfigured() bool {
	return true
}

func (s *DevSandboxStorageService) ProviderName() string {
	return "dev_sandbox"
}

func (s *DevSandboxStorageService) GeneratePresignedUpload(ctx context.Context, key string, contentType string, maxBytes int64, expiry time.Duration) (string, error) {
	// Returns a simulated presigned upload URL referencing the dev sandbox
	expiresUnix := time.Now().Add(expiry).Unix()
	return fmt.Sprintf("%s/api/v1/marketplace/uploads/mock-presign?key=%s&expires=%d", s.publicDomain, key, expiresUnix), nil
}

func (s *DevSandboxStorageService) UploadObject(ctx context.Context, key string, contentType string, reader io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read object data: %w", err)
	}

	s.mu.Lock()
	s.objects[key] = storedObject{
		data:        data,
		contentType: contentType,
		uploadedAt:  time.Now(),
	}
	s.mu.Unlock()

	return s.GetPublicURL(key), nil
}

func (s *DevSandboxStorageService) DeleteObject(ctx context.Context, key string) error {
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

func (s *DevSandboxStorageService) GetPublicURL(key string) string {
	cleanKey := strings.TrimPrefix(key, "/")
	return fmt.Sprintf("%s/%s", s.publicDomain, cleanKey)
}

func (s *DevSandboxStorageService) ReadObject(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, exists := s.objects[key]
	if !exists {
		return nil, ErrObjectNotFound
	}
	return obj.data, nil
}

func (s *DevSandboxStorageService) GetObject(key string) ([]byte, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, exists := s.objects[key]
	if !exists {
		return nil, "", false
	}
	return obj.data, obj.contentType, true
}

// -------------------------------------------------------------
// UnconfiguredStorageService (Production safety fallback)
// -------------------------------------------------------------

type UnconfiguredStorageService struct{}

func NewUnconfiguredStorageService() *UnconfiguredStorageService {
	return &UnconfiguredStorageService{}
}

func (s *UnconfiguredStorageService) IsConfigured() bool {
	return false
}

func (s *UnconfiguredStorageService) ProviderName() string {
	return "unconfigured"
}

func (s *UnconfiguredStorageService) GeneratePresignedUpload(ctx context.Context, key string, contentType string, maxBytes int64, expiry time.Duration) (string, error) {
	return "", ErrStorageUnconfigured
}

func (s *UnconfiguredStorageService) UploadObject(ctx context.Context, key string, contentType string, reader io.Reader, size int64) (string, error) {
	return "", ErrStorageUnconfigured
}

func (s *UnconfiguredStorageService) ReadObject(ctx context.Context, key string) ([]byte, error) {
	return nil, ErrStorageUnconfigured
}

func (s *UnconfiguredStorageService) DeleteObject(ctx context.Context, key string) error {
	return ErrStorageUnconfigured
}

func (s *UnconfiguredStorageService) GetPublicURL(key string) string {
	return ""
}

