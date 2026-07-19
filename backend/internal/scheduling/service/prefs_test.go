package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
	"github.com/stretchr/testify/require"
)

type fakePrefs struct{ saved domain.SchedulerPrefs }

func (f *fakePrefs) Get(_ context.Context, _ db.Querier, uid uuid.UUID) (domain.SchedulerPrefs, error) {
	if f.saved.UserID == uid {
		return f.saved, nil
	}
	p := domain.DefaultPrefs()
	p.UserID = uid
	return p, nil
}
func (f *fakePrefs) Upsert(_ context.Context, _ db.Querier, p domain.SchedulerPrefs) error {
	f.saved = p
	return nil
}

func TestPrefsService_UpdateRejectsOutOfRange(t *testing.T) {
	svc := service.NewPrefsService(nil, &fakePrefs{})
	_, err := svc.Update(context.Background(), uuid.New(), service.PrefsUpdate{DesiredRetention: 0.5, Timezone: "UTC"})
	require.ErrorIs(t, err, service.ErrRetentionRange)
}

func TestPrefsService_UpdateRejectsBadTimezone(t *testing.T) {
	svc := service.NewPrefsService(nil, &fakePrefs{})
	_, err := svc.Update(context.Background(), uuid.New(), service.PrefsUpdate{DesiredRetention: 0.9, Timezone: "Mars/Phobos"})
	require.ErrorIs(t, err, service.ErrBadTimezone)
}

func TestPrefsService_UpdatePersists(t *testing.T) {
	fp := &fakePrefs{}
	svc := service.NewPrefsService(nil, fp)
	uid := uuid.New()
	got, err := svc.Update(context.Background(), uid, service.PrefsUpdate{
		DesiredRetention: 0.85, DailyNewLimit: 30, DailyReviewLimit: 150, Timezone: "Asia/Bangkok",
	})
	require.NoError(t, err)
	require.InDelta(t, 0.85, got.DesiredRetention, 1e-9)
	require.Equal(t, "Asia/Bangkok", fp.saved.Timezone)
}
