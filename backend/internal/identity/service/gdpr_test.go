package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestExportData_RequiresReauth(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "g@example.com", Password: "Tr0ub4dour!", DisplayName: "G",
	})
	// sai mật khẩu → từ chối (re-auth, NFR-14)
	if _, err := h.svc.ExportData(context.Background(), res.UserID, "wrong-pass"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("export must re-auth, got %v", err)
	}
	exp, err := h.svc.ExportData(context.Background(), res.UserID, "Tr0ub4dour!")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exp.User.Email != "g@example.com" || exp.User.DisplayName != "G" {
		t.Errorf("export payload incomplete: %+v", exp.User)
	}
	if exp.ExportedAt.IsZero() {
		t.Error("export must be timestamped")
	}
}

func TestDeleteAccount_SoftDeletesAndRevokes(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "d@example.com", Password: "Tr0ub4dour!"})
	login, _ := h.svc.Login(context.Background(), "d@example.com", "Tr0ub4dour!")

	if err := h.svc.DeleteAccount(context.Background(), res.UserID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// user biến khỏi lookup theo email (soft-delete)
	if _, err := h.stores.Users.ByEmail(context.Background(), "d@example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Error("soft-deleted user must not resolve by email")
	}
	// session bị thu hồi
	if _, err := h.svc.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("sessions must be revoked on account deletion (Story 1.8)")
	}
}
