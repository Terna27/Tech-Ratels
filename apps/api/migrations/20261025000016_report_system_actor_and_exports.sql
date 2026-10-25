-- +goose Up
-- Migration 16: Report System Actor Attribution and Private Storage Exports Extension
-- Fixes system attribution on public_report_snapshots and expands public_report_exports metadata.

-- 1. Snapshot System Actor Attribution
-- Allow system-generated snapshots to have NULL created_by_user_id while maintaining foreign key integrity for human users.
ALTER TABLE public_report_snapshots ALTER COLUMN created_by_user_id DROP NOT NULL;

-- Add created_by_type to explicitly identify whether a snapshot was initiated by an administrator or the automated system.
ALTER TABLE public_report_snapshots ADD COLUMN IF NOT EXISTS created_by_type VARCHAR(30) NOT NULL DEFAULT 'administrator';

CREATE INDEX IF NOT EXISTS idx_report_snapshots_created_by_type ON public_report_snapshots(created_by_type);

-- 2. Expand public_report_exports for storage integrity, checksum verification, and deduplication
ALTER TABLE public_report_exports ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE public_report_exports ADD COLUMN IF NOT EXISTS mime_type VARCHAR(100) NOT NULL DEFAULT 'application/octet-stream';
ALTER TABLE public_report_exports ADD COLUMN IF NOT EXISTS checksum_algorithm VARCHAR(30) NOT NULL DEFAULT 'sha256';
ALTER TABLE public_report_exports ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Unique constraint ensuring one active export per (snapshot_id, export_format)
DROP INDEX IF EXISTS idx_report_exports_snapshot;
CREATE UNIQUE INDEX IF NOT EXISTS idx_report_exports_snapshot_format ON public_report_exports(snapshot_id, export_format);

-- +goose Down
DROP INDEX IF EXISTS idx_report_exports_snapshot_format;
CREATE INDEX IF NOT EXISTS idx_report_exports_snapshot ON public_report_exports(snapshot_id, export_format);

ALTER TABLE public_report_exports DROP COLUMN IF EXISTS updated_at;
ALTER TABLE public_report_exports DROP COLUMN IF EXISTS checksum_algorithm;
ALTER TABLE public_report_exports DROP COLUMN IF EXISTS mime_type;
ALTER TABLE public_report_exports DROP COLUMN IF EXISTS file_size_bytes;

DROP INDEX IF EXISTS idx_report_snapshots_created_by_type;
ALTER TABLE public_report_snapshots DROP COLUMN IF EXISTS created_by_type;
