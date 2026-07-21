package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/shared/events"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeSched struct{}

func (fakeSched) Apply(c scheddom.Card, g scheddom.Grade, r float64, now time.Time) scheddom.ScheduleResult {
	return scheddom.ScheduleResult{Stability: 5, Difficulty: 5, Status: scheddom.StatusReview,
		Reps: c.Reps + 1, Lapses: c.Lapses, DueAt: now.AddDate(0, 0, 5), LastReviewAt: now, Retrievability: 0.9}
}
func (fakeSched) Preview(scheddom.Card, float64, time.Time) scheddom.NextIntervals {
	return scheddom.NextIntervals{}
}

type fakeCards struct{ applied int }

func (c *fakeCards) Load(_ context.Context, _ db.Querier, cardID, owner uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{ID: cardID, OwnerID: owner, Status: scheddom.StatusNew}, nil
}
func (c *fakeCards) ApplyResult(_ context.Context, _ db.Querier, _ uuid.UUID, _ scheddom.ScheduleResult) error {
	c.applied++
	return nil
}
func (c *fakeCards) DueCards(context.Context, db.Querier, uuid.UUID, time.Time, int) ([]scheddom.Card, error) {
	return nil, nil
}

type fakePrefs struct{}

func (fakePrefs) Get(context.Context, db.Querier, uuid.UUID) (scheddom.SchedulerPrefs, error) {
	return scheddom.DefaultPrefs(), nil
}
func (fakePrefs) Upsert(context.Context, db.Querier, scheddom.SchedulerPrefs) error { return nil }

type fakeLogs struct {
	mu   sync.Mutex
	rows []revdom.ReviewLogRow
}

func (l *fakeLogs) Append(_ context.Context, _ db.Querier, row revdom.ReviewLogRow) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, row)
	return nil
}
func (l *fakeLogs) ListForOwnerSince(context.Context, db.Querier, uuid.UUID, string) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}
func (l *fakeLogs) ListForCard(context.Context, db.Querier, uuid.UUID) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}

type fakeReceipts struct {
	mu    sync.Mutex
	store map[string]revdom.GradeResult
}

func newFakeReceipts() *fakeReceipts { return &fakeReceipts{store: map[string]revdom.GradeResult{}} }
func key(card uuid.UUID, cr string) string { return card.String() + "|" + cr }

func (r *fakeReceipts) Insert(_ context.Context, _ db.Querier, res revdom.GradeResult, _ uuid.UUID, cr string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(res.CardID, cr)
	if _, ok := r.store[k]; ok {
		return false, nil
	}
	r.store[k] = res
	return true, nil
}
func (r *fakeReceipts) Get(_ context.Context, _ db.Querier, card uuid.UUID, cr string) (revdom.GradeResult, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.store[key(card, cr)]
	return res, ok, nil
}

// txRunner fake: chạy fn ngay với Querier=nil (không cần DB thật cho unit test).
func directTx(_ context.Context, fn func(db.Querier) error) error { return fn(nil) }

func newSvc(bus *eventbus.InProcess) (*service.GradeService, *fakeCards, *fakeLogs) {
	cards := &fakeCards{}
	logs := &fakeLogs{}
	svc := service.NewGradeService(service.GradeDeps{
		Tx: directTx, Scheduler: fakeSched{}, Cards: cards, Prefs: fakePrefs{},
		Logs: logs, Receipts: newFakeReceipts(), Bus: bus,
		Clock: func() time.Time { return time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC) },
	})
	return svc, cards, logs
}

func TestGrade_ServerComputesAndPersistsOnce(t *testing.T) {
	bus := eventbus.NewInProcess()
	var mu sync.Mutex
	count := 0
	var got events.CardGraded
	bus.Subscribe(events.CardGradedName, func(_ context.Context, ev eventbus.Event) {
		mu.Lock()
		defer mu.Unlock()
		count++
		got = ev.Payload.(events.CardGraded) // hợp đồng chung, không phải kiểu review-local
	})

	svc, cards, logs := newSvc(bus)
	owner := uuid.New()
	cmd := revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.GradeGood, ClientReviewID: "cr-1"}
	res, err := svc.Grade(context.Background(), owner, cmd)
	require.NoError(t, err)
	require.InDelta(t, 5, res.Stability, 1e-9) // server tính, không nhận từ client
	require.Equal(t, 1, cards.applied)
	require.Len(t, logs.rows, 1)
	bus.Wait()
	require.Equal(t, 1, count)
	// payload = shared/events.CardGraded, đủ field để progress ingest (AD-8).
	require.Equal(t, owner.String(), got.OwnerID)
	require.Equal(t, cmd.CardID.String(), got.CardID)
	require.Equal(t, int(scheddom.GradeGood), got.Grade)
	require.Equal(t, 5, got.ScheduledDays) // fakeSched: due = now + 5 ngày
	require.True(t, got.WasNew)            // card load Status=StatusNew trước khi chấm
	require.Equal(t, time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC), got.ReviewedAt)
}

func TestGrade_IdempotentOnDuplicateClientReviewID(t *testing.T) {
	bus := eventbus.NewInProcess()
	var mu sync.Mutex
	count := 0
	bus.Subscribe(events.CardGradedName, func(context.Context, eventbus.Event) { mu.Lock(); count++; mu.Unlock() })

	svc, cards, logs := newSvc(bus)
	owner := uuid.New()
	cmd := revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.GradeGood, ClientReviewID: "cr-dup"}

	r1, err := svc.Grade(context.Background(), owner, cmd)
	require.NoError(t, err)
	r2, err := svc.Grade(context.Background(), owner, cmd) // gửi lại y hệt
	require.NoError(t, err)

	require.Equal(t, r1, r2, "trả kết quả cũ")
	require.Equal(t, 1, cards.applied, "KHÔNG chấm lại card")
	require.Len(t, logs.rows, 1, "KHÔNG tạo log trùng (AD-4)")
	bus.Wait()
	require.Equal(t, 1, count, "KHÔNG phát event lần hai")
}

func TestGrade_RejectsInvalidGrade(t *testing.T) {
	svc, _, _ := newSvc(eventbus.NewInProcess())
	_, err := svc.Grade(context.Background(), uuid.New(),
		revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.Grade(9), ClientReviewID: "x"})
	require.ErrorIs(t, err, service.ErrInvalidGrade)
}
