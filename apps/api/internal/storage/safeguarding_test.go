package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestSafeguardingStorage_MockStoreAndRead(t *testing.T) {
	svc := NewDevSafeguardingStorageService()
	ctx := context.Background()

	caseID := "c1000000-0000-0000-0000-000000000001"
	content := "This is confidential evidence document content for safeguarding investigation."
	reader := strings.NewReader(content)

	key, sha256Hex, size, err := svc.StoreEvidence(ctx, caseID, "incident_statement.txt", "text/plain", reader, int64(len(content)))
	if err != nil {
		t.Fatalf("unexpected error storing evidence: %v", err)
	}

	if !strings.HasPrefix(key, "safeguarding/cases/c1000000-0000-0000-0000-000000000001/evidence/") {
		t.Errorf("unexpected storage key prefix: %s", key)
	}
	if len(sha256Hex) != 64 {
		t.Errorf("expected 64-char sha256 hex, got: %s", sha256Hex)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	// Read evidence back
	data, err := svc.ReadEvidence(ctx, key)
	if err != nil {
		t.Fatalf("failed to read back evidence: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected data %q, got %q", content, string(data))
	}

	// Generate download URL
	downloadURL, err := svc.GeneratePresignedDownload(ctx, key, "statement.txt", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate download URL: %v", err)
	}
	if !strings.Contains(downloadURL, key) {
		t.Errorf("expected download URL to contain key, got: %s", downloadURL)
	}

	// Delete evidence
	err = svc.DeleteEvidence(ctx, key)
	if err != nil {
		t.Fatalf("failed to delete evidence: %v", err)
	}

	_, err = svc.ReadEvidence(ctx, key)
	if err != ErrObjectNotFound {
		t.Errorf("expected ErrObjectNotFound after deletion, got: %v", err)
	}
}

func TestSafeguardingStorage_RejectsDisallowedFiles(t *testing.T) {
	svc := NewDevSafeguardingStorageService()
	ctx := context.Background()
	caseID := "c1000000-0000-0000-0000-000000000001"

	disallowed := []struct {
		name string
		mime string
	}{
		{"exploit.exe", "application/octet-stream"},
		{"malicious.sh", "text/x-shellscript"},
		{"payload.bat", "text/plain"},
		{"script.php", "application/x-php"},
		{"malware.bin", "application/octet-stream"},
	}

	for _, tc := range disallowed {
		_, _, _, err := svc.StoreEvidence(ctx, caseID, tc.name, tc.mime, strings.NewReader("dummy payload"), 13)
		if err != ErrDisallowedFileType {
			t.Errorf("expected ErrDisallowedFileType for %s (%s), got: %v", tc.name, tc.mime, err)
		}
	}
}

func TestSafeguardingStorage_RejectsOversizedFile(t *testing.T) {
	svc := NewDevSafeguardingStorageService()
	ctx := context.Background()
	caseID := "c1000000-0000-0000-0000-000000000001"

	// 26 MB declared
	oversized := int64(26 * 1024 * 1024)
	_, _, _, err := svc.StoreEvidence(ctx, caseID, "large_video.mp4", "video/mp4", bytes.NewReader([]byte("dummy")), oversized)
	if err != ErrEvidenceTooLarge {
		t.Errorf("expected ErrEvidenceTooLarge, got: %v", err)
	}
}

func TestSafeguardingStorage_RejectsEmptyPayload(t *testing.T) {
	svc := NewDevSafeguardingStorageService()
	ctx := context.Background()
	caseID := "c1000000-0000-0000-0000-000000000001"

	_, _, _, err := svc.StoreEvidence(ctx, caseID, "empty.png", "image/png", strings.NewReader(""), 0)
	if err != ErrEvidenceEmpty {
		t.Errorf("expected ErrEvidenceEmpty, got: %v", err)
	}
}

func TestSafeguardingStorage_UnconfiguredR2(t *testing.T) {
	r2 := NewR2SafeguardingStorage(nil, nil, "")
	if r2.IsConfigured() {
		t.Error("expected IsConfigured to return false for nil client")
	}

	ctx := context.Background()
	_, _, _, err := r2.StoreEvidence(ctx, "c1", "test.jpg", "image/jpeg", strings.NewReader("data"), 4)
	if err != ErrSafeguardingUnconfig {
		t.Errorf("expected ErrSafeguardingUnconfig, got: %v", err)
	}
}
