package domain

import "github.com/google/uuid"

// CuratedDeck là bộ thẻ khởi đầu seed sẵn (FR-11a, AD-6).
type CuratedDeck struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	Description string
	IsActive    bool
}
