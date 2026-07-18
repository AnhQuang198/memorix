package ports_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// schedAdapter chứng minh scheduling.Service thỏa CardService (ráp thật ở cmd/api).
type schedAdapter struct{ *schedsvc.Service }

func (a schedAdapter) CreateCardsForEntry(ctx context.Context, in ports.CreateCardsInput) error {
	return a.Service.CreateCardsForEntry(ctx, schedsvc.CreateCardsInput(in))
}

func TestSchedulingServiceSatisfiesPort(t *testing.T) {
	var _ ports.CardService = schedAdapter{}
	_ = uuid.Nil
}
