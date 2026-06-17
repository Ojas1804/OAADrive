CREATE TYPE upload_status AS ENUM ('pending', 'in_progress', 'completed', 'cancelled', 'failed');

CREATE TABLE upload_sessions (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bucket       VARCHAR(100)  NOT NULL,
    object_key   TEXT          NOT NULL,
    file_name    TEXT          NOT NULL,
    total_size   BIGINT        NOT NULL DEFAULT 0,
    upload_id    TEXT          NOT NULL DEFAULT '',
    status       upload_status NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW() + INTERVAL '24 hours',
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_upload_sessions_user_id ON upload_sessions (user_id);
CREATE INDEX idx_upload_sessions_status  ON upload_sessions (status);
