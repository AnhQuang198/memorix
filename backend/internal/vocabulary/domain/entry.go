package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Direction là hướng ôn tập của thẻ (front→back hoặc back→front).
type Direction string

const (
	DirectionFrontBack Direction = "front_back"
	DirectionBackFront Direction = "back_front"
)

func (d Direction) Valid() bool {
	return d == DirectionFrontBack || d == DirectionBackFront
}

// DefaultDirections áp mặc định front→back (FR-8); lọc giá trị hợp lệ.
func DefaultDirections(in []Direction) []Direction {
	var out []Direction
	for _, d := range in {
		if d.Valid() {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return []Direction{DirectionFrontBack}
	}
	return out
}

type Relation string

const (
	RelationSynonym Relation = "synonym"
	RelationAntonym Relation = "antonym"
)

type Meaning struct {
	ID           uuid.UUID
	PartOfSpeech string
	Definition   string
	Position     int
}

type Example struct {
	ID       uuid.UUID
	Text     string
	Position int
}

type Pronunciation struct {
	ID       uuid.UUID
	IPA      string
	Dialect  string
	AudioURL string
}

type SynAnt struct {
	ID       uuid.UUID
	Relation Relation
	Value    string
}

// Entry giữ nội dung từ (AD-6). OwnerID nil = curated (owner_id NULL).
type Entry struct {
	ID             uuid.UUID
	OwnerID        *uuid.UUID
	CuratedDeckID  *uuid.UUID
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Meanings       []Meaning
	Examples       []Example
	Pronunciations []Pronunciation
	Relations      []SynAnt
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// ValidateTerm trim + bắt buộc không rỗng (FR-7).
func ValidateTerm(term string) (string, error) {
	t := strings.TrimSpace(term)
	if t == "" {
		return "", ErrTermRequired
	}
	return t, nil
}
