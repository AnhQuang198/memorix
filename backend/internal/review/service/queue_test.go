package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

type queueCards struct{ cards []scheddom.Card }

func (q *queueCards) Load(context.Context, db.Querier, uuid.UUID, uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{}, nil
}
func (q *queueCards) ApplyResult(context.Context, db.Querier, uuid.UUID, scheddom.ScheduleResult) error {
	return nil
}
func (q *queueCards) DueCards(_ context.Context, _ db.Querier, _ uuid.UUID, _ time.Time, _ int) ([]scheddom.Card, error) {
	return q.cards, nil
}

type prevSched struct{}

func (prevSched) Apply(scheddom.Card, scheddom.Grade, float64, time.Time) scheddom.ScheduleResult {
	return scheddom.ScheduleResult{}
}
func (prevSched) Preview(_ scheddom.Card, _ float64, _ time.Time) scheddom.NextIntervals {
	return scheddom.NextIntervals{
		Again: 10 * time.Minute, Hard: 24 * time.Hour, Good: 4 * 24 * time.Hour, Easy: 9 * 24 * time.Hour,
	}
}

func TestQueue_BuildsItemsWithContentAndIntervals(t *testing.T) {
	owner := uuid.New()
	entry := uuid.New()
	card := scheddom.Card{ID: uuid.New(), OwnerID: owner, EntryID: entry, Direction: "front_back"}

	vocab := revports.VocabularyFunc(func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]revports.EntryContent, error) {
		require.Equal(t, []uuid.UUID{entry}, ids, "batch-load đúng entry ids")
		return map[uuid.UUID]revports.EntryContent{
			entry: {EntryID: entry, Term: "ephemeral", IPA: "/ɪ'fem(ə)rəl/", Meaning: "chóng tàn", Example: "an ephemeral trend"},
		}, nil
	})

	svc := service.NewQueueService(service.QueueDeps{
		Pool: nil, RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Cards: &queueCards{cards: []scheddom.Card{card}}, Prefs: fakePrefs{}, Scheduler: prevSched{}, Vocab: vocab,
		Clock: func() time.Time { return time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC) },
	})

	items, err := svc.Queue(context.Background(), owner, 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "ephemeral", items[0].Term)
	require.Equal(t, int64(600), items[0].NextIntervals.AgainSeconds) // 10 phút
	require.Equal(t, int64(9*86400), items[0].NextIntervals.EasySeconds)
}

func TestQueue_SkipsCardsWithMissingContent(t *testing.T) {
	owner := uuid.New()
	card := scheddom.Card{ID: uuid.New(), OwnerID: owner, EntryID: uuid.New()}
	vocab := revports.VocabularyFunc(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]revports.EntryContent, error) {
		return map[uuid.UUID]revports.EntryContent{}, nil // không có content
	})
	svc := service.NewQueueService(service.QueueDeps{
		RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Cards:    &queueCards{cards: []scheddom.Card{card}}, Prefs: fakePrefs{}, Scheduler: prevSched{}, Vocab: vocab,
		Clock: func() time.Time { return time.Now() },
	})
	items, err := svc.Queue(context.Background(), owner, 50)
	require.NoError(t, err)
	require.Empty(t, items, "thẻ thiếu nội dung bị bỏ khỏi queue")
}
