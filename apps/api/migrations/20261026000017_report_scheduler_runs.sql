-- +goose Up
-- Migration 17: Durable Report Scheduler Execution Run History
-- Persists end-to-end execution records of automated and manually triggered report generation cycles.

CREATE TABLE IF NOT EXISTS public_report_scheduler_runs (
    id VARCHAR(36) PRIMARY KEY,
    run_identifier VARCHAR(64) NOT NULL UNIQUE,
    trigger_source VARCHAR(50) NOT NULL, -- 'scheduled', 'admin_manual', 'external_cron'
    reporting_period VARCHAR(50) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'running', -- 'running', 'completed', 'partial_failure', 'failed'
    reports_evaluated INT NOT NULL DEFAULT 0,
    reports_generated INT NOT NULL DEFAULT 0,
    reports_skipped INT NOT NULL DEFAULT 0,
    reports_failed INT NOT NULL DEFAULT 0,
    generated_references JSONB NOT NULL DEFAULT '[]'::jsonb,
    skipped_reasons JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_details JSONB NOT NULL DEFAULT '[]'::jsonb,
    actor_type VARCHAR(30) NOT NULL DEFAULT 'system', -- 'system', 'administrator'
    actor_user_id VARCHAR(36) REFERENCES users(id),
    actor_name VARCHAR(150),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_sched_runs_status ON public_report_scheduler_runs(status);
CREATE INDEX IF NOT EXISTS idx_report_sched_runs_period ON public_report_scheduler_runs(reporting_period);
CREATE INDEX IF NOT EXISTS idx_report_sched_runs_trigger ON public_report_scheduler_runs(trigger_source);
CREATE INDEX IF NOT EXISTS idx_report_sched_runs_started_at ON public_report_scheduler_runs(started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS public_report_scheduler_runs;
