// Package service điều phối read model Progress (module nhẹ: service + repo).
package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

// IngestRepo — ghi read model từ event (write side).
type IngestRepo interface {
	BumpDailyStat(ctx context.Context, ownerID string, day domain.Day, wasNew bool, grade int, retained bool) error
	GetStudyProfile(ctx context.Context, userID string) (domain.StudyProfile, bool, error)
	UpsertStudyProfile(ctx context.Context, userID string, p domain.StudyProfile) error
}

// TZResolver phân giải TZ user cho "ngày học" (AD-12). MVP background mặc định UTC;
// prod wire IdentityPort (deferred).
type TZResolver interface {
	Location(ctx context.Context, userID string) *time.Location
}

// UTCResolver mặc định UTC.
type UTCResolver struct{}

func (UTCResolver) Location(context.Context, string) *time.Location { return time.UTC }
