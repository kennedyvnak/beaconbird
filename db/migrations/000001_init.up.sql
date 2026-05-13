CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    mode TEXT NOT NULL CHECK (mode IN ('live', 'test')),
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    channel TEXT NOT NULL CHECK (channel IN ('email', 'push')),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id TEXT PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'cancelled')),
    tag TEXT NOT NULL DEFAULT '',
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    send_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at TIMESTAMPTZ
);

CREATE TABLE delivery_jobs (
    id TEXT PRIMARY KEY,
    notification_id TEXT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    channel TEXT NOT NULL CHECK (channel IN ('email', 'push')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'retrying', 'dead_letter', 'cancelled')),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    send_at TIMESTAMPTZ NOT NULL,
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    heartbeat_at TIMESTAMPTZ,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE delivery_attempts (
    id TEXT PRIMARY KEY,
    delivery_job_id TEXT NOT NULL REFERENCES delivery_jobs(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    provider_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'sent', 'failed', 'retrying', 'dead_letter', 'cancelled')),
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    duration_ms INTEGER,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT delivery_attempts_job_attempt_unique UNIQUE (delivery_job_id, attempt_number)
);

CREATE INDEX idx_api_keys_active ON api_keys (is_revoked, mode);
CREATE INDEX idx_notifications_status_send_at ON notifications (status, send_at);
CREATE INDEX idx_notifications_tag_created_at ON notifications (tag, created_at DESC);
CREATE INDEX idx_delivery_jobs_ready_pending ON delivery_jobs (status, send_at) WHERE status = 'pending';
CREATE INDEX idx_delivery_jobs_ready_retrying ON delivery_jobs (status, next_retry_at) WHERE status = 'retrying';
CREATE INDEX idx_delivery_jobs_notification_id ON delivery_jobs (notification_id);
CREATE INDEX idx_delivery_jobs_locked_by ON delivery_jobs (locked_by) WHERE locked_by IS NOT NULL;
CREATE INDEX idx_delivery_jobs_stale_lock ON delivery_jobs (heartbeat_at) WHERE locked_at IS NOT NULL;
CREATE INDEX idx_delivery_attempts_job_attempted_at ON delivery_attempts (delivery_job_id, attempted_at DESC);
CREATE INDEX idx_providers_channel_active ON providers (channel, is_active);
