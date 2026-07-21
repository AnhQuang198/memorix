-- Chỉ drop object do 0007 sở hữu. idx_cards_owner_due thuộc 0004_cards (expand-contract)
-- nên KHÔNG drop ở đây — 0007 up chỉ CREATE IF NOT EXISTS (no-op), không sở hữu index.
DROP TABLE IF EXISTS scheduling.study_profiles;
