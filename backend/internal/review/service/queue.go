package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

// QueueIntervals = next_intervals dạng giây (client format thành "10 phút"/"4 ngày").
type QueueIntervals struct {
	AgainSeconds int64 `json:"again_seconds"`
	HardSeconds  int64 `json:"hard_seconds"`
	GoodSeconds  int64 `json:"good_seconds"`
	EasySeconds  int64 `json:"easy_seconds"`
}

// QueueItem = 1 thẻ đến hạn kèm nội dung mặt sau + khoảng cách ôn kế.
type QueueItem struct {
	CardID        uuid.UUID      `json:"card_id"`
	EntryID       uuid.UUID      `json:"entry_id"`
	Direction     string         `json:"direction"`
	Term          string         `json:"term"`
	IPA           string         `json:"ipa"`
	Meaning       string         `json:"meaning"`
	Example       string         `json:"example"`
	NextIntervals QueueIntervals `json:"next_intervals"`
}

// QueryRunner chạy fn với 1 Querier read-only.
type QueryRunner func(ctx context.Context, fn func(db.Querier) error) error

type QueueDeps struct {
	Pool      *pgxpool.Pool
	RunQuery  QueryRunner
	Cards     schedports.CardStore
	Prefs     schedports.PrefsStore
	Scheduler schedports.SchedulerPort
	Vocab     revports.VocabularyPort
	Clock     func() time.Time
}

type QueueService struct{ d QueueDeps }

func NewQueueService(d QueueDeps) *QueueService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.RunQuery == nil && d.Pool != nil {
		p := d.Pool
		d.RunQuery = func(_ context.Context, fn func(db.Querier) error) error { return fn(p) }
	}
	return &QueueService{d: d}
}

func (s *QueueService) Queue(ctx context.Context, ownerID uuid.UUID, limit int) ([]QueueItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	now := s.d.Clock()

	var (
		cards     []cardsnapshot
		retention float64
	)
	err := s.d.RunQuery(ctx, func(q db.Querier) error {
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		retention = prefs.DesiredRetention
		due, err := s.d.Cards.DueCards(ctx, q, ownerID, now, limit)
		if err != nil {
			return err
		}
		for _, c := range due {
			iv := s.d.Scheduler.Preview(c, retention, now)
			cards = append(cards, cardsnapshot{card: c, iv: iv})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return []QueueItem{}, nil
	}

	ids := make([]uuid.UUID, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.card.EntryID)
	}
	content, err := s.d.Vocab.BatchGet(ctx, ownerID, ids)
	if err != nil {
		return nil, err
	}

	items := make([]QueueItem, 0, len(cards))
	for _, c := range cards {
		ec, ok := content[c.card.EntryID]
		if !ok {
			continue // thiếu nội dung → bỏ khỏi queue (không hiển thị thẻ rỗng)
		}
		items = append(items, QueueItem{
			CardID: c.card.ID, EntryID: c.card.EntryID, Direction: string(c.card.Direction),
			Term: ec.Term, IPA: ec.IPA, Meaning: ec.Meaning, Example: ec.Example,
			NextIntervals: QueueIntervals{
				AgainSeconds: secs(c.iv.Again), HardSeconds: secs(c.iv.Hard),
				GoodSeconds: secs(c.iv.Good), EasySeconds: secs(c.iv.Easy),
			},
		})
	}
	return items, nil
}

type cardsnapshot struct {
	card cardType
	iv   intervalType
}

func secs(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return int64(d / time.Second)
}
