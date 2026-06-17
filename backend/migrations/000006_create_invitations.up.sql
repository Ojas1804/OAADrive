CREATE TABLE invitations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email       VARCHAR(255) NOT NULL,
    role        user_role   NOT NULL DEFAULT 'member',
    token_hash  VARCHAR(64) NOT NULL UNIQUE,
    invited_by  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invitations_email      ON invitations (email);
CREATE INDEX idx_invitations_token_hash ON invitations (token_hash);
