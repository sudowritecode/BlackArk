CREATE TABLE IF NOT EXISTS join_tokens (
    token_hash BYTEA PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS resources JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS logs TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS deployments_app_slot_idx ON deployments(app_id, id);
