-- Bộ khởi đầu IELTS seed (FR-11a, AD-6: curated owner_id NULL). Idempotent qua slug.
INSERT INTO vocabulary.curated_decks (id, slug, name, description)
VALUES ('00000000-0000-0000-0000-0000000d0001', 'ielts-starter', 'IELTS Starter',
        'Bộ từ vựng IELTS khởi đầu để bắt đầu học ngay (chống cold-start).')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term, part_of_speech, source)
SELECT NULL, '00000000-0000-0000-0000-0000000d0001', t.term, t.pos, 'ielts-starter'
FROM (VALUES
    ('ubiquitous','adj'),
    ('meticulous','adj'),
    ('pragmatic','adj'),
    ('resilient','adj'),
    ('ambiguous','adj'),
    ('coherent','adj'),
    ('inevitable','adj'),
    ('profound','adj')
) AS t(term, pos)
WHERE NOT EXISTS (
    SELECT 1 FROM vocabulary.entries e
    WHERE e.curated_deck_id = '00000000-0000-0000-0000-0000000d0001'
      AND e.term_normalized = vocabulary.immutable_unaccent(lower(t.term))
);

INSERT INTO vocabulary.meanings (entry_id, part_of_speech, definition, position)
SELECT e.id, 'adj', d.def, 0
FROM vocabulary.entries e
JOIN (VALUES
    ('ubiquitous','present, appearing, or found everywhere'),
    ('meticulous','showing great attention to detail; very careful'),
    ('pragmatic','dealing with things sensibly and realistically'),
    ('resilient','able to recover quickly from difficulties'),
    ('ambiguous','open to more than one interpretation; unclear'),
    ('coherent','logical and consistent'),
    ('inevitable','certain to happen; unavoidable'),
    ('profound','very great or intense; showing deep insight')
) AS d(term, def) ON e.term = d.term
WHERE e.curated_deck_id = '00000000-0000-0000-0000-0000000d0001'
  AND NOT EXISTS (SELECT 1 FROM vocabulary.meanings m WHERE m.entry_id = e.id);
