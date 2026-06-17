CREATE TABLE files (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bucket     VARCHAR(100) NOT NULL,
    object_key TEXT         NOT NULL,
    file_name  TEXT         NOT NULL,
    size       BIGINT       NOT NULL DEFAULT 0,
    checksum   VARCHAR(64)  NOT NULL,
    mime_type  VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (bucket, object_key)
);

CREATE INDEX idx_files_owner_id  ON files (owner_id);
CREATE INDEX idx_files_bucket    ON files (bucket);
CREATE INDEX idx_files_checksum  ON files (checksum);
