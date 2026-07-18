-- Vocabulary schema (AD-6 Entry tách Card; AD-10 FK chỉ trong cùng schema).

-- Wrapper IMMUTABLE để dùng unaccent trong generated column / index.
-- unaccent extension cài ở public (Sprint 0). Chỉ định regdictionary để immutable-safe.
CREATE OR REPLACE FUNCTION vocabulary.immutable_unaccent(text)
RETURNS text LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS
$$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;

CREATE TABLE vocabulary.curated_decks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE vocabulary.entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid,                                       -- NULL = curated (AD-6); ref logic identity.users (AD-10)
    curated_deck_id uuid REFERENCES vocabulary.curated_decks(id) ON DELETE CASCADE, -- FK cùng schema OK
    term            text NOT NULL,
    term_normalized text GENERATED ALWAYS AS (vocabulary.immutable_unaccent(lower(term))) STORED,
    part_of_speech  text NOT NULL DEFAULT '',
    notes           text NOT NULL DEFAULT '',
    source          text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

-- Trùng theo (owner, term chuẩn hóa) chỉ tính bản chưa xóa (FR-10).
CREATE UNIQUE INDEX uq_entries_owner_termnorm
    ON vocabulary.entries (owner_id, term_normalized)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_entries_owner_created
    ON vocabulary.entries (owner_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_entries_curated_deck
    ON vocabulary.entries (curated_deck_id)
    WHERE curated_deck_id IS NOT NULL AND deleted_at IS NULL;

-- gin FTS index (yêu cầu sprint; phục vụ search list ?q=).
CREATE INDEX idx_entries_fts
    ON vocabulary.entries
    USING gin (to_tsvector('english', term || ' ' || notes));

CREATE TABLE vocabulary.meanings (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id       uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    part_of_speech text NOT NULL DEFAULT '',
    definition     text NOT NULL,
    position       int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_meanings_entry ON vocabulary.meanings(entry_id);

CREATE TABLE vocabulary.examples (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    text     text NOT NULL,
    position int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_examples_entry ON vocabulary.examples(entry_id);

CREATE TABLE vocabulary.pronunciations (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id  uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    ipa       text NOT NULL DEFAULT '',
    dialect   text NOT NULL DEFAULT '',
    audio_url text NOT NULL DEFAULT ''
);
CREATE INDEX idx_pron_entry ON vocabulary.pronunciations(entry_id);

CREATE TABLE vocabulary.synonyms_antonyms (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    relation text NOT NULL CHECK (relation IN ('synonym','antonym')),
    value    text NOT NULL
);
CREATE INDEX idx_synant_entry ON vocabulary.synonyms_antonyms(entry_id);

CREATE TABLE vocabulary.deck_enrollments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid NOT NULL,                              -- ref logic identity.users
    curated_deck_id uuid NOT NULL REFERENCES vocabulary.curated_decks(id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed')),
    card_count      int  NOT NULL DEFAULT 0,
    enrolled_at     timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz
);
CREATE UNIQUE INDEX uq_enrollment_owner_deck
    ON vocabulary.deck_enrollments (owner_id, curated_deck_id);
