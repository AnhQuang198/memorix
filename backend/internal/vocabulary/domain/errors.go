package domain

import "errors"

var (
	ErrTermRequired    = errors.New("term is required")
	ErrEntryNotFound   = errors.New("entry not found")
	ErrDeckNotFound    = errors.New("curated deck not found")
	ErrAlreadyEnrolled = errors.New("already enrolled in deck")
	ErrDuplicateTerm   = errors.New("duplicate term for owner")
)
