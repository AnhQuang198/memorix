package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestWindow_AllowsUpToLimitThenDenies(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	w := NewWindow(3, time.Minute)
	w.now = func() time.Time { return base }
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, _ := w.Allow(ctx, "login:a@b.com")
		if !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if ok, _ := w.Allow(ctx, "login:a@b.com"); ok {
		t.Error("4th attempt in window must be denied")
	}
	// khóa khác không bị ảnh hưởng
	if ok, _ := w.Allow(ctx, "login:other@b.com"); !ok {
		t.Error("different key must be independent")
	}
}

func TestWindow_ResetClearsKey(t *testing.T) {
	w := NewWindow(1, time.Minute)
	ctx := context.Background()
	_, _ = w.Allow(ctx, "k")
	if ok, _ := w.Allow(ctx, "k"); ok {
		t.Fatal("should be denied after limit")
	}
	w.Reset(ctx, "k")
	if ok, _ := w.Allow(ctx, "k"); !ok {
		t.Error("Reset must clear the key (login success path)")
	}
}

func TestWindow_ExpiresAfterWindow(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	w := NewWindow(1, time.Minute)
	w.now = func() time.Time { return base }
	ctx := context.Background()
	_, _ = w.Allow(ctx, "k")
	w.now = func() time.Time { return base.Add(61 * time.Second) }
	if ok, _ := w.Allow(ctx, "k"); !ok {
		t.Error("attempt must be allowed after window elapsed")
	}
}
