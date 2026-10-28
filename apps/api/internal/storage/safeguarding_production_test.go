package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProductionEvidenceNeverFallsBackToMemory(t *testing.T) {
	ctx := context.Background()
	s, err := NewEnvironmentSafeguardingStorage(ctx, true, R2Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s.IsConfigured() {
		t.Fatal("production evidence reports ephemeral storage as configured")
	}
	_, _, _, err = s.StoreEvidence(ctx, "case", "evidence.txt", "text/plain", strings.NewReader("evidence"), 8)
	if !errors.Is(err, ErrSafeguardingUnconfig) {
		t.Fatalf("must reject evidence instead of losing it on restart: %v", err)
	}
	s, err = NewEnvironmentSafeguardingStorage(ctx, true, R2Config{AccountID: "test", AccessKeyID: "test", SecretAccessKey: "test", BucketName: "private-evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*R2SafeguardingStorage); !ok || !s.IsConfigured() {
		t.Fatal("configured production evidence must use durable R2 storage")
	}
	s, err = NewEnvironmentSafeguardingStorage(ctx, false, R2Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*DevSafeguardingStorageService); !ok {
		t.Fatal("development sandbox unavailable")
	}
}
