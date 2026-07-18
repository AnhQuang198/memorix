package service

import (
	"context"
	"testing"
	"time"
)

func TestPurgeDeletedAccounts_RemovesOldSoftDeletes(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "old@example.com", Password: "Tr0ub4dour!"})
	// soft-delete tại thời điểm quá khứ
	h.clock.t = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := h.svc.DeleteAccount(context.Background(), res.UserID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// purge mọi tài khoản xóa trước 2026-06-15 (retention 14 ngày)
	n, err := h.svc.PurgeDeletedAccounts(context.Background(), 14*24*time.Hour, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 account purged, got %d", n)
	}
}

func TestPurgeDeletedAccounts_KeepsRecent(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "recent@example.com", Password: "Tr0ub4dour!"})
	h.clock.t = time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	_ = h.svc.DeleteAccount(context.Background(), res.UserID)
	// xóa mới 1 ngày trước, retention 14d → chưa purge
	n, err := h.svc.PurgeDeletedAccounts(context.Background(), 14*24*time.Hour, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 0 {
		t.Errorf("recent soft-delete must be retained, purged %d", n)
	}
}
