-- Runs automatically the FIRST time the postgres container initializes
-- an empty data directory. If you change this file after the volume
-- already has data, it will NOT re-run automatically — you'd need to
-- apply changes as a migration instead.

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member', -- 'admin' or 'member'
    storage_quota_bytes BIGINT NOT NULL DEFAULT 5368709120, -- 5 GB default
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id),
    original_filename TEXT NOT NULL,
    storage_bucket TEXT NOT NULL,
    storage_object_key TEXT NOT NULL,
    size_bytes         BIGINT NOT NULL,
    mime_type TEXT,
    checksum_sha256 TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE backup_jobs (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id),
    source_path TEXT NOT NULL,
    schedule TEXT, -- e.g. a cron expression
    last_run_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    action TEXT NOT NULL,
    target TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_owner ON files(owner_id);
CREATE INDEX idx_backup_jobs_owner ON backup_jobs(owner_id);
