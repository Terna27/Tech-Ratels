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
