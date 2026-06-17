CREATE TABLE family_members (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    joined_at  TIMESTAMPTZ,
    UNIQUE (admin_id, user_id)
);

CREATE INDEX idx_family_members_admin_id ON family_members (admin_id);
CREATE INDEX idx_family_members_user_id  ON family_members (user_id);
