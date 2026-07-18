-- Story 1.2/1.3/1.4/1.5/1.8 — bảng module identity. FK CHỈ trong schema identity (AD-10).

CREATE TABLE identity.users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             citext NOT NULL,
    password_hash     text NOT NULL DEFAULT '',
    display_name      text NOT NULL DEFAULT '',
    timezone          text NOT NULL DEFAULT 'UTC',
    locale            text NOT NULL DEFAULT 'vi',
    theme             text NOT NULL DEFAULT 'system',
    email_verified_at timestamptz,
    plan              text NOT NULL DEFAULT 'free',
    role              text NOT NULL DEFAULT 'user',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz
);
CREATE UNIQUE INDEX users_email_active_uniq ON identity.users (email) WHERE deleted_at IS NULL;

CREATE TABLE identity.email_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('verify','reset')),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_tokens_user_idx ON identity.email_tokens (user_id);

CREATE TABLE identity.sessions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    family_id          uuid NOT NULL,
    refresh_token_hash text NOT NULL UNIQUE,
    rotated_to         uuid REFERENCES identity.sessions(id),
    expires_at         timestamptz NOT NULL,
    revoked_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_family_idx ON identity.sessions (family_id);
CREATE INDEX sessions_user_idx ON identity.sessions (user_id);

CREATE TABLE identity.oauth_identities (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    provider     text NOT NULL,
    provider_uid text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);
CREATE INDEX oauth_identities_user_idx ON identity.oauth_identities (user_id);
