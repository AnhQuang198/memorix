package repo

import (
	"context"

	"github.com/google/uuid"
)

// ReviewContent = nội dung tối thiểu của một entry cho mặt sau thẻ review
// (batch-load, AD-9). Chỉ các field queue cần; không kéo toàn bộ bảng con.
type ReviewContent struct {
	EntryID uuid.UUID
	Term    string
	IPA     string
	Meaning string
	Example string
}

// BatchReviewContent batch-load nội dung entry cho queue review theo tập id. Bao
// gồm cả entry của owner LẪN entry curated (owner_id NULL — thẻ enroll trỏ tới,
// AD-6); mỗi entry lấy IPA/nghĩa/ví dụ đầu tiên (theo position). Bỏ entry đã xóa.
func (r *Repo) BatchReviewContent(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]ReviewContent, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.term,
		  COALESCE((SELECT p.ipa FROM vocabulary.pronunciations p
		            WHERE p.entry_id = e.id ORDER BY p.id LIMIT 1), ''),
		  COALESCE((SELECT m.definition FROM vocabulary.meanings m
		            WHERE m.entry_id = e.id ORDER BY m.position, m.id LIMIT 1), ''),
		  COALESCE((SELECT x.text FROM vocabulary.examples x
		            WHERE x.entry_id = e.id ORDER BY x.position, x.id LIMIT 1), '')
		FROM vocabulary.entries e
		WHERE e.id = ANY($2) AND e.deleted_at IS NULL
		  AND (e.owner_id = $1 OR e.owner_id IS NULL)`,
		ownerID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReviewContent
	for rows.Next() {
		var rc ReviewContent
		if err := rows.Scan(&rc.EntryID, &rc.Term, &rc.IPA, &rc.Meaning, &rc.Example); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}
