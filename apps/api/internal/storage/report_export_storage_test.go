package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/storage"
)

func TestReportExportStorage_DevSandboxLifecycle(t *testing.T) {
	st := storage.NewDevSandboxReportExportStorage()
	ctx := context.Background()

	if !st.IsConfigured() {
		t.Fatalf("expected sandbox storage to be configured")
	}

	payload := []byte("%PDF-1.4 sample pdf content for export testing")
	snapshotID := "snap-uuid-12345"

	// 1. Store export
	key, sha, size, err := st.StoreExport(ctx, snapshotID, "pdf", "Report_Jan.pdf", "application/pdf", payload)
	if err != nil {
		t.Fatalf("StoreExport failed: %v", err)
	}

	if !strings.HasPrefix(key, "reports/exports/private/") {
		t.Errorf("expected private prefix, got %q", key)
	}
	if size != int64(len(payload)) {
		t.Errorf("expected size %d, got %d", len(payload), size)
	}

	hasher := sha256.New()
	hasher.Write(payload)
	expectedSha := hex.EncodeToString(hasher.Sum(nil))
	if sha != expectedSha {
		t.Errorf("expected sha %s, got %s", expectedSha, sha)
	}

	// 2. Read export
	readData, err := st.ReadExport(ctx, key)
	if err != nil {
		t.Fatalf("ReadExport failed: %v", err)
	}
	if string(readData) != string(payload) {
		t.Errorf("read data mismatch: expected %q, got %q", string(payload), string(readData))
	}

	// 3. Generate presigned download
	presignedURL, err := st.GeneratePresignedDownload(ctx, key, "download.pdf", 15*time.Minute)
	if err != nil {
		t.Fatalf("GeneratePresignedDownload failed: %v", err)
	}
	if !strings.Contains(presignedURL, key) {
		t.Errorf("presigned URL does not contain storage key: %s", presignedURL)
	}

	// 4. Delete export
	err = st.DeleteExport(ctx, key)
	if err != nil {
		t.Fatalf("DeleteExport failed: %v", err)
	}

	// 5. Subsequent read should fail
	_, err = st.ReadExport(ctx, key)
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound after deletion, got %v", err)
	}
}

func TestReportExportStorage_ValidationRules(t *testing.T) {
	st := storage.NewDevSandboxReportExportStorage()
	ctx := context.Background()

	// Empty payload
	_, _, _, err := st.StoreExport(ctx, "snap-1", "pdf", "file.pdf", "application/pdf", []byte{})
	if !errors.Is(err, storage.ErrExportEmpty) {
		t.Errorf("expected ErrExportEmpty for empty bytes, got %v", err)
	}

	// Path traversal in Read
	_, err = st.ReadExport(ctx, "reports/exports/private/../../etc/passwd")
	if !errors.Is(err, storage.ErrInvalidExportKey) {
		t.Errorf("expected ErrInvalidExportKey for traversal path, got %v", err)
	}

	// Path traversal in Presigned
	_, err = st.GeneratePresignedDownload(ctx, "reports/exports/private/snap/../../../secret", "file.pdf", 10*time.Minute)
	if !errors.Is(err, storage.ErrInvalidExportKey) {
		t.Errorf("expected ErrInvalidExportKey for traversal presigned, got %v", err)
	}

	// Missing private prefix
	_, err = st.ReadExport(ctx, "public/images/logo.png")
	if !errors.Is(err, storage.ErrInvalidExportKey) {
		t.Errorf("expected ErrInvalidExportKey for foreign prefix, got %v", err)
	}
}

func TestReportExportStorage_CollisionResistance(t *testing.T) {
	st := storage.NewDevSandboxReportExportStorage()
	ctx := context.Background()

	payload := []byte("identical binary content")
	key1, _, _, err1 := st.StoreExport(ctx, "snap-same", "csv", "data.csv", "text/csv", payload)
	key2, _, _, err2 := st.StoreExport(ctx, "snap-same", "csv", "data.csv", "text/csv", payload)

	if err1 != nil || err2 != nil {
		t.Fatalf("storage error: %v, %v", err1, err2)
	}
	if key1 == key2 {
		t.Errorf("expected collision-resistant unique keys, got identical keys: %s", key1)
	}
}

func TestReportExportStorage_UnconfiguredFallback(t *testing.T) {
	st := storage.NewUnconfiguredReportExportStorage()
	ctx := context.Background()

	if st.IsConfigured() {
		t.Fatal("UnconfiguredReportExportStorage must return false for IsConfigured")
	}

	_, _, _, err := st.StoreExport(ctx, "snap-1", "pdf", "file.pdf", "application/pdf", []byte("sample"))
	if !errors.Is(err, storage.ErrReportExportUnconfig) {
		t.Errorf("expected ErrReportExportUnconfig on StoreExport, got %v", err)
	}

	_, err = st.ReadExport(ctx, "reports/exports/private/snap-1/pdf/1_file.pdf")
	if !errors.Is(err, storage.ErrReportExportUnconfig) {
		t.Errorf("expected ErrReportExportUnconfig on ReadExport, got %v", err)
	}

	_, err = st.GeneratePresignedDownload(ctx, "reports/exports/private/snap-1/pdf/1_file.pdf", "file.pdf", 15*time.Minute)
	if !errors.Is(err, storage.ErrReportExportUnconfig) {
		t.Errorf("expected ErrReportExportUnconfig on GeneratePresignedDownload, got %v", err)
	}

	err = st.DeleteExport(ctx, "reports/exports/private/snap-1/pdf/1_file.pdf")
	if !errors.Is(err, storage.ErrReportExportUnconfig) {
		t.Errorf("expected ErrReportExportUnconfig on DeleteExport, got %v", err)
	}
}

func TestReportExportStorage_SeparateCredentialsAndIsolatedClients(t *testing.T) {
	ctx := context.Background()

	// Marketplace configuration with public bucket 'ratels' and marketplace credentials
	cfgMarket := storage.R2Config{
		AccountID:       "cf_acc_111",
		AccessKeyID:     "mkt_access_key_111",
		SecretAccessKey: "mkt_secret_key_111",
		BucketName:      "ratels",
		PublicDomain:    "https://media.ratels.example.org",
	}

	// Report storage configuration with isolated private bucket 'ratels-reports-private' and dedicated reports credentials
	cfgReports := storage.R2Config{
		AccountID:       "cf_acc_111",
		AccessKeyID:     "rep_access_key_222",
		SecretAccessKey: "rep_secret_key_222",
		BucketName:      "ratels-reports-private",
	}

	// Verify configs are completely separate
	if cfgMarket.BucketName == cfgReports.BucketName {
		t.Fatalf("marketplace and reports buckets must be distinct")
	}
	if cfgMarket.AccessKeyID == cfgReports.AccessKeyID {
		t.Fatalf("marketplace and reports access keys must be distinct")
	}
	if cfgMarket.SecretAccessKey == cfgReports.SecretAccessKey {
		t.Fatalf("marketplace and reports secret keys must be distinct")
	}

	// Verify both storage clients initialize properly with their respective configs
	mktClient, err := storage.NewR2StorageService(ctx, cfgMarket)
	if err != nil {
		t.Fatalf("failed creating marketplace R2 client: %v", err)
	}
	if !mktClient.IsConfigured() {
		t.Errorf("expected marketplace client to be configured")
	}

	repClient, err := storage.NewR2ReportExportStorageFromConfig(ctx, cfgReports)
	if err != nil {
		t.Fatalf("failed creating report export R2 client: %v", err)
	}
	if !repClient.IsConfigured() {
		t.Errorf("expected report export client to be configured")
	}
}

func TestReportExportStorage_IndependentMarketplaceAndReportsLifecycle(t *testing.T) {
	ctx := context.Background()

	// 1. Marketplace dev storage
	mktStorage := storage.NewDevSandboxStorageService("https://pub-media.ratels.example.org")
	// 2. Reports dev storage
	repStorage := storage.NewDevSandboxReportExportStorage()

	// Marketplace product image upload
	imgPayload := []byte("FAKE_IMAGE_DATA_FOR_MARKETPLACE_PRODUCT")
	url, err := mktStorage.UploadObject(ctx, "products/prod_123/thumbnail.jpg", "image/jpeg", strings.NewReader(string(imgPayload)), int64(len(imgPayload)))
	if err != nil {
		t.Fatalf("marketplace upload failed: %v", err)
	}
	if !strings.Contains(url, "products/prod_123/thumbnail.jpg") {
		t.Errorf("expected product image URL, got: %s", url)
	}

	// Report private PDF upload
	pdfPayload := []byte("%PDF-1.4 CERTIFIED PUBLIC GOVERNANCE REPORT")
	storageKey, sha256Hex, size, err := repStorage.StoreExport(ctx, "snap-lifecycle-999", "pdf", "Report.pdf", "application/pdf", pdfPayload)
	if err != nil {
		t.Fatalf("report storage failed: %v", err)
	}
	if size != int64(len(pdfPayload)) || len(sha256Hex) != 64 {
		t.Errorf("invalid size or sha256: size=%d, sha=%s", size, sha256Hex)
	}

	// Verify marketplace storage does not know about the private report
	_, mktReadErr := mktStorage.ReadObject(ctx, storageKey)
	if mktReadErr == nil {
		t.Fatalf("marketplace storage must not have access to private report object")
	}

	// Verify report storage does not know about marketplace product image
	_, repReadErr := repStorage.ReadExport(ctx, "products/prod_123/thumbnail.jpg")
	if repReadErr == nil {
		t.Fatalf("report storage must not accept or contain marketplace product image")
	}

	// Verify report storage reads back private report correctly
	readPDF, err := repStorage.ReadExport(ctx, storageKey)
	if err != nil || string(readPDF) != string(pdfPayload) {
		t.Fatalf("report storage failed reading back private PDF: err=%v", err)
	}
}
