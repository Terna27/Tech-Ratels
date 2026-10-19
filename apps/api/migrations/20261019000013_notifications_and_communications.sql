-- +goose Up
-- Phase B: Notifications & Communications Engine Migration

-- 1. In-App Notifications
CREATE TABLE IF NOT EXISTS notifications (
    id VARCHAR(36) PRIMARY KEY,
    recipient_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(50) NOT NULL, -- 'cleanup_reminder', 'event_rescheduled', 'claim_approved', 'claim_rejected', 'leadership_assigned', 'leadership_revoked', 'media_approved', 'media_rejected', 'media_revision', 'order_status', 'career_status', 'project_status', 'finance_action', 'announcement'
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    related_resource_type VARCHAR(50), -- 'event', 'claim', 'appointment', 'media', 'order', 'application', 'project', 'finance'
    related_resource_id VARCHAR(36),
    action_url VARCHAR(500),
    priority VARCHAR(20) NOT NULL DEFAULT 'normal', -- 'low', 'normal', 'high', 'urgent'
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_created ON notifications(recipient_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_unread ON notifications(recipient_id) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(notification_type);

-- 2. Transactional Notification Outbox (Durable queue for asynchronous background delivery)
CREATE TABLE IF NOT EXISTS notification_outbox (
    id VARCHAR(36) PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    recipient_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    recipient_email VARCHAR(255),
    recipient_phone VARCHAR(50),
    recipient_name VARCHAR(150),
    channel VARCHAR(20) NOT NULL, -- 'in_app', 'email', 'sms'
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    delivery_state VARCHAR(30) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'delivered', 'failed', 'dead_letter', 'skipped'
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lease_token VARCHAR(64),
    leased_until TIMESTAMPTZ,
    last_error TEXT,
    last_error_category VARCHAR(50), -- 'network', 'timeout', 'auth_error', 'validation_error', 'provider_unconfigured', 'rate_limited', 'opt_out'
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_poll ON notification_outbox(delivery_state, next_attempt_at) WHERE delivery_state = 'pending';
CREATE INDEX IF NOT EXISTS idx_outbox_recipient ON notification_outbox(recipient_id);
CREATE INDEX IF NOT EXISTS idx_outbox_channel_state ON notification_outbox(channel, delivery_state);
CREATE INDEX IF NOT EXISTS idx_outbox_created ON notification_outbox(created_at DESC);

-- 3. Notification Deliveries (Audit history & provider response tracking)
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id VARCHAR(36) PRIMARY KEY,
    outbox_id VARCHAR(36) NOT NULL REFERENCES notification_outbox(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- 'in_app', 'brevo', 'termii', 'dev_sandbox', 'unconfigured'
    channel VARCHAR(20) NOT NULL, -- 'in_app', 'email', 'sms'
    provider_message_ref VARCHAR(255),
    status VARCHAR(30) NOT NULL, -- 'accepted', 'delivered', 'rejected', 'failed', 'skipped'
    failure_category VARCHAR(50),
    error_message TEXT,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    response_payload TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_deliveries_outbox_id ON notification_deliveries(outbox_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_created_at ON notification_deliveries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_provider ON notification_deliveries(provider, status);

-- 4. Notification Preferences
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id VARCHAR(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    in_app_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sms_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    cleanup_reminders_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    leadership_alerts_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    participation_updates_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    marketplace_updates_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    careers_updates_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    community_project_updates_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    finance_alerts_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    announcements_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sms_consent_recorded_at TIMESTAMPTZ,
    sms_consent_withdrawn_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. Notification Templates
CREATE TABLE IF NOT EXISTS notification_templates (
    id VARCHAR(36) PRIMARY KEY,
    template_code VARCHAR(100) NOT NULL UNIQUE,
    channel VARCHAR(20) NOT NULL, -- 'in_app', 'email', 'sms'
    title_template VARCHAR(255) NOT NULL,
    body_template TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 6. Notification Schedules (Automated cleanup reminders with unique deduplication)
CREATE TABLE IF NOT EXISTS notification_schedules (
    id VARCHAR(36) PRIMARY KEY,
    schedule_type VARCHAR(50) NOT NULL, -- 'cleanup_reminder_7d', 'cleanup_reminder_1d'
    event_id VARCHAR(36) NOT NULL REFERENCES event_instances(id) ON DELETE CASCADE,
    event_schedule_version INTEGER NOT NULL DEFAULT 1,
    target_time TIMESTAMPTZ NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- 'pending', 'dispatched', 'cancelled', 'superseded'
    dispatched_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_notification_schedules UNIQUE(schedule_type, event_id, event_schedule_version)
);

CREATE INDEX IF NOT EXISTS idx_schedules_poll ON notification_schedules(status, target_time) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_schedules_event ON notification_schedules(event_id);

-- Seed Baseline Templates
INSERT INTO notification_templates (id, template_code, channel, title_template, body_template, version, is_active)
VALUES
    ('t1000000-0000-0000-0000-000000000001', 'cleanup_reminder_7d_email', 'email', 'Upcoming Ratels Cleanup in 7 Days: {{.EventTitle}}', 'Hello {{.RecipientName}}, this is a reminder that the {{.EventTitle}} cleanup is scheduled for {{.EventDate}} at {{.EventLocation}}. Check your gear and assemble with your squad!', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000002', 'cleanup_reminder_7d_sms', 'sms', 'Cleanup in 7 Days', 'Ratels Alert: {{.EventTitle}} is in 7 days ({{.EventDate}} at {{.EventLocation}}). Together for a cleaner Nigeria!', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000003', 'cleanup_reminder_7d_in_app', 'in_app', 'Upcoming Cleanup in 7 Days', 'The {{.EventTitle}} cleanup is coming up in 7 days on {{.EventDate}}.', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000004', 'cleanup_reminder_1d_email', 'email', 'Tomorrow: Ratels Cleanup Campaign - {{.EventTitle}}', 'Hello {{.RecipientName}}, tomorrow is the nationwide Ratels cleanup for {{.EventTitle}} at {{.EventLocation}} starting at {{.StartTime}}. See you on the ground!', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000005', 'cleanup_reminder_1d_sms', 'sms', 'Cleanup Tomorrow', 'Ratels Alert: Tomorrow is {{.EventTitle}} starting at {{.StartTime}} at {{.EventLocation}}. Be ready!', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000006', 'cleanup_reminder_1d_in_app', 'in_app', 'Cleanup Campaign Tomorrow', 'Reminder: {{.EventTitle}} starts tomorrow at {{.StartTime}} at {{.EventLocation}}.', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000007', 'event_rescheduled_email', 'email', 'Schedule Update: {{.EventTitle}}', 'Hello {{.RecipientName}}, please note that {{.EventTitle}} has been rescheduled to {{.NewDate}}. Reason: {{.Reason}}', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000008', 'event_rescheduled_sms', 'sms', 'Schedule Change', 'Ratels Notice: {{.EventTitle}} has been rescheduled to {{.NewDate}}. Check details at {{.ActionURL}}', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000009', 'event_rescheduled_in_app', 'in_app', 'Event Rescheduled', '{{.EventTitle}} has been rescheduled to {{.NewDate}}.', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000010', 'participation_verified_email', 'email', 'Your Cleanup Attendance has been Verified!', 'Congratulations {{.RecipientName}}! Your participation in {{.EventTitle}} has been officially verified by coordinators.', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000011', 'claim_rejected_email', 'email', 'Update regarding your participation claim', 'Hello {{.RecipientName}}, your participation claim for {{.EventTitle}} could not be verified. Reason: {{.Reason}}. You may submit an appeal if needed.', 1, TRUE),
    ('t1000000-0000-0000-0000-000000000012', 'media_approved_email', 'email', 'Your cleanup media has been approved', 'Hello {{.RecipientName}}, your photos/videos for {{.EventTitle}} have been approved and published to the public gallery.', 1, TRUE)
ON CONFLICT (template_code) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS notification_schedules;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_outbox;
DROP TABLE IF EXISTS notifications;
