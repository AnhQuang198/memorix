package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func ptr(s string) *string { return &s }

func TestUpdateProfile_AppliesFields(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	u, err := h.svc.UpdateProfile(context.Background(), res.UserID, ProfileInput{
		DisplayName: ptr("Minh"), Timezone: ptr("Asia/Ho_Chi_Minh"),
		Locale: ptr("en"), Theme: ptr("dark"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if u.DisplayName != "Minh" || u.Timezone != "Asia/Ho_Chi_Minh" || u.Locale != "en" || u.Theme != "dark" {
		t.Errorf("profile not applied: %+v", u)
	}
	// persisted
	got, _ := h.stores.Users.ByID(context.Background(), res.UserID)
	if got.Timezone != "Asia/Ho_Chi_Minh" {
		t.Error("timezone not persisted (needed for study-day AD-12)")
	}
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	u, err := h.svc.UpdateProfile(context.Background(), res.UserID, ProfileInput{Theme: ptr("light")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if u.Theme != "light" || u.Locale != "vi" {
		t.Errorf("partial update should keep defaults: %+v", u)
	}
}

func TestUpdateProfile_InvalidValuesRejected(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	for _, in := range []ProfileInput{
		{Timezone: ptr("Mars/Phobos")},
		{Locale: ptr("fr")},
		{Theme: ptr("neon")},
	} {
		if _, err := h.svc.UpdateProfile(context.Background(), res.UserID, in); !errors.Is(err, domain.ErrInvalidProfile) {
			t.Errorf("expected ErrInvalidProfile for %+v, got %v", in, err)
		}
	}
}
