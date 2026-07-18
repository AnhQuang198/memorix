// Package ports expose hợp đồng vocabulary cần từ module khác (AD-1, AD-9).
// CardService định nghĩa ở phía gọi (vocabulary) để tránh import cycle với
// scheduling; scheduling.Service hiện thực, ráp ở cmd/api.
package ports

import (
	"context"

	"github.com/google/uuid"
)

type CreateCardsInput struct {
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Directions []string
}

// CardService là cổng vocabulary → scheduling. Vocabulary KHÔNG ghi thẳng
// scheduling.cards; mọi thao tác card đi qua đây (AD-9, AD-10).
type CardService interface {
	CreateCardsForEntry(ctx context.Context, in CreateCardsInput) error
	CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error)
	EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error)
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}
