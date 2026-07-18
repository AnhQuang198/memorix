package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/domain"
)

func TestPgRepos_UserRoundTripAndSessionRotation(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	repos := New(pool)

	now := time.Now().UTC().Truncate(time.Second)
	u := &domain.User{
		ID: uuid.NewString(), Email: "int@example.com", PasswordHash: "h:x",
		Timezone: "UTC", Locale: "vi", Theme: "system",
		Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now,
	}
	if err := repos.Users.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := repos.Users.ByEmail(ctx, "INT@example.com") // citext case-insensitive
	if err != nil || got.ID != u.ID {
		t.Fatalf("ByEmail citext: got=%+v err=%v", got, err)
	}

	// email verified update
	got.EmailVerifiedAt = &now
	got.UpdatedAt = now
	if err := repos.Users.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := repos.Users.ByID(ctx, u.ID)
	if !reloaded.IsVerified() {
		t.Error("email_verified_at not persisted")
	}

	// session create + rotate
	fam := uuid.NewString()
	s1 := &domain.Session{ID: uuid.NewString(), UserID: u.ID, FamilyID: fam,
		RefreshTokenHash: "hash-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	s2 := &domain.Session{ID: uuid.NewString(), UserID: u.ID, FamilyID: fam,
		RefreshTokenHash: "hash-2", ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := repos.Sessions.Create(ctx, s1); err != nil {
		t.Fatalf("session1: %v", err)
	}
	if err := repos.Sessions.Create(ctx, s2); err != nil {
		t.Fatalf("session2: %v", err)
	}
	if err := repos.Sessions.MarkRotated(ctx, s1.ID, s2.ID, now); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	back, _ := repos.Sessions.ByTokenHash(ctx, "hash-1")
	if back.RevokedAt == nil || back.RotatedTo == nil || *back.RotatedTo != s2.ID {
		t.Errorf("rotation not persisted: %+v", back)
	}
	// revoke family → s2 revoked
	if err := repos.Sessions.RevokeFamily(ctx, fam, now); err != nil {
		t.Fatalf("revoke family: %v", err)
	}
	s2back, _ := repos.Sessions.ByTokenHash(ctx, "hash-2")
	if s2back.RevokedAt == nil {
		t.Error("family revoke did not reach successor session")
	}
}

func TestPgRepos_EmailTokenAndPurge(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	pool, _ := pgxpool.New(ctx, dsn)
	defer pool.Close()
	repos := New(pool)

	now := time.Now().UTC().Truncate(time.Second)
	u := &domain.User{ID: uuid.NewString(), Email: "tok@example.com",
		Timezone: "UTC", Locale: "vi", Theme: "system",
		Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now}
	_ = repos.Users.Create(ctx, u)

	tok := &domain.EmailToken{ID: uuid.NewString(), UserID: u.ID, Kind: domain.KindVerify,
		TokenHash: "th-1", ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now}
	if err := repos.Tokens.Create(ctx, tok); err != nil {
		t.Fatalf("token create: %v", err)
	}
	got, err := repos.Tokens.ByTokenHash(ctx, "th-1", domain.KindVerify)
	if err != nil || got.ID != tok.ID {
		t.Fatalf("token lookup: %v", err)
	}
	if err := repos.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	// purge: soft-delete rồi purge trước cutoff
	_ = repos.Users.SoftDelete(ctx, u.ID, now.Add(-48*time.Hour))
	n, err := repos.Users.PurgeDeletedBefore(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged, got %d", n)
	}
}
