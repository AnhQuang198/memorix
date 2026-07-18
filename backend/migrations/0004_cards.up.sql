-- Card = trạng thái học per-user, per-direction (AD-6). entry_id/owner_id là ref
-- logic (không FK chéo schema, AD-10). Sprint 2 chỉ tạo card New; FSRS = Epic 3.
CREATE TABLE scheduling.cards (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL,                                   -- ref logic identity.users
    entry_id   uuid NOT NULL,                                   -- ref logic vocabulary.entries
    direction  text NOT NULL DEFAULT 'front_back' CHECK (direction IN ('front_back','back_front')),
    status     text NOT NULL DEFAULT 'new' CHECK (status IN ('new','learning','review','relearning','suspended')),
    due_at     timestamptz,
    stability  double precision NOT NULL DEFAULT 0,
    difficulty double precision NOT NULL DEFAULT 0,
    reps       int NOT NULL DEFAULT 0,
    lapses     int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT uq_cards_owner_entry_dir UNIQUE (owner_id, entry_id, direction)
);
CREATE INDEX idx_cards_owner_status ON scheduling.cards (owner_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_cards_owner_entry  ON scheduling.cards (owner_id, entry_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_cards_owner_due    ON scheduling.cards (owner_id, due_at) WHERE deleted_at IS NULL;
