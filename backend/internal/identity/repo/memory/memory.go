package memory

import (
	"context"
	"sync"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Stores gom 4 repo in-memory thread-safe cho unit test và dev.
type Stores struct {
	Users    *UserStore
	Sessions *SessionStore
	Tokens   *TokenStore
	OAuth    *OAuthStore
}

func New() *Stores {
	return &Stores{
		Users:    &UserStore{byID: map[string]domain.User{}},
		Sessions: &SessionStore{byID: map[string]domain.Session{}},
		Tokens:   &TokenStore{byID: map[string]domain.EmailToken{}},
		OAuth:    &OAuthStore{byID: map[string]domain.OAuthIdentity{}},
	}
}

// --- Users ---

type UserStore struct {
	mu   sync.Mutex
	byID map[string]domain.User
}

func (s *UserStore) Create(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[u.ID] = *u
	return nil
}

func (s *UserStore) ByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = domain.NormalizeEmail(email)
	for _, u := range s.byID {
		if u.DeletedAt == nil && domain.NormalizeEmail(u.Email) == email {
			cp := u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *UserStore) ByID(_ context.Context, id string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := u
	return &cp, nil
}

func (s *UserStore) Update(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[u.ID]; !ok {
		return domain.ErrNotFound
	}
	s.byID[u.ID] = *u
	return nil
}

func (s *UserStore) SoftDelete(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.DeletedAt = &at
	u.UpdatedAt = at
	s.byID[id] = u
	return nil
}

func (s *UserStore) PurgeDeletedBefore(_ context.Context, cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, u := range s.byID {
		if u.DeletedAt != nil && u.DeletedAt.Before(cutoff) {
			delete(s.byID, id)
			n++
		}
	}
	return n, nil
}

// --- Sessions ---

type SessionStore struct {
	mu   sync.Mutex
	byID map[string]domain.Session
}

func (s *SessionStore) Create(_ context.Context, sess *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sess.ID] = *sess
	return nil
}

func (s *SessionStore) ByTokenHash(_ context.Context, hash string) (*domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.byID {
		if sess.RefreshTokenHash == hash {
			cp := sess
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *SessionStore) MarkRotated(_ context.Context, id, successorID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	sess.RotatedTo = &successorID
	sess.RevokedAt = &at
	s.byID[id] = sess
	return nil
}

func (s *SessionStore) RevokeFamily(_ context.Context, familyID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.FamilyID == familyID && sess.RevokedAt == nil {
			sess.RevokedAt = &at
			s.byID[id] = sess
		}
	}
	return nil
}

func (s *SessionStore) RevokeAllForUser(_ context.Context, userID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.UserID == userID && sess.RevokedAt == nil {
			sess.RevokedAt = &at
			s.byID[id] = sess
		}
	}
	return nil
}

// --- Email tokens ---

type TokenStore struct {
	mu   sync.Mutex
	byID map[string]domain.EmailToken
}

func (s *TokenStore) Create(_ context.Context, t *domain.EmailToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[t.ID] = *t
	return nil
}

func (s *TokenStore) ByTokenHash(_ context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.byID {
		if t.TokenHash == hash && t.Kind == kind {
			cp := t
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *TokenStore) MarkUsed(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	t.UsedAt = &at
	s.byID[id] = t
	return nil
}

// --- OAuth identities ---

type OAuthStore struct {
	mu   sync.Mutex
	byID map[string]domain.OAuthIdentity
}

func (s *OAuthStore) ByProviderUID(_ context.Context, provider, uid string) (*domain.OAuthIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.byID {
		if o.Provider == provider && o.ProviderUID == uid {
			cp := o
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *OAuthStore) Create(_ context.Context, o *domain.OAuthIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[o.ID] = *o
	return nil
}

// Compile-time checks: memory stores thỏa ports.
var (
	_ ports.UserRepo       = (*UserStore)(nil)
	_ ports.SessionRepo    = (*SessionStore)(nil)
	_ ports.EmailTokenRepo = (*TokenStore)(nil)
	_ ports.OAuthRepo      = (*OAuthStore)(nil)
)
