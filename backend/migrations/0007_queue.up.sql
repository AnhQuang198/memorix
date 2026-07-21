-- Sprint 4 — Smart Queue. NFR-2 + cờ coach luồng học thẻ mới (Story 4.5).

-- NFR-2: index nóng phục vụ BuildQueue (overdue/due theo owner), p95<500ms cho 10k thẻ.
-- idx_cards_owner_due đã do 0004_cards tạo (partial WHERE deleted_at IS NULL) và 0006
-- re-assert IF NOT EXISTS. Ở đây IF NOT EXISTS ⇒ no-op an toàn, đảm bảo index tồn tại
-- kể cả khi chạy migration set tối thiểu (guard NFR-2, không tạo trùng).
CREATE INDEX IF NOT EXISTS idx_cards_owner_due
	ON scheduling.cards (owner_id, due_at);

-- Cờ "đã xem hướng dẫn chấm điểm lần đầu" cho luồng học thẻ mới (FR-29, Story 4.5)
-- + scaffolding streak. FK chỉ trong schema (AD-10): user_id là ref logic identity.users,
-- KHÔNG FK chéo schema.
CREATE TABLE IF NOT EXISTS scheduling.study_profiles (
	user_id             uuid PRIMARY KEY,
	learn_coach_seen_at timestamptz,
	created_at          timestamptz NOT NULL DEFAULT now(),
	updated_at          timestamptz NOT NULL DEFAULT now()
);
