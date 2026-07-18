package domain

import "testing"

func TestValidateTerm(t *testing.T) {
	got, err := ValidateTerm("  hello  ")
	if err != nil || got != "hello" {
		t.Fatalf("ValidateTerm trim = %q, %v", got, err)
	}
	if _, err := ValidateTerm("   "); err != ErrTermRequired {
		t.Errorf("blank term err = %v, want ErrTermRequired", err)
	}
}

func TestDirectionValid(t *testing.T) {
	if !DirectionFrontBack.Valid() || !DirectionBackFront.Valid() {
		t.Error("known directions must be valid")
	}
	if Direction("sideways").Valid() {
		t.Error("unknown direction must be invalid")
	}
}

func TestDefaultDirections(t *testing.T) {
	got := DefaultDirections(nil)
	if len(got) != 1 || got[0] != DirectionFrontBack {
		t.Fatalf("default = %v, want [front_back]", got)
	}
	both := DefaultDirections([]Direction{DirectionFrontBack, DirectionBackFront})
	if len(both) != 2 {
		t.Errorf("passthrough = %v", both)
	}
	if bad := DefaultDirections([]Direction{"x"}); len(bad) != 1 || bad[0] != DirectionFrontBack {
		t.Errorf("invalid direction should fall back to default, got %v", bad)
	}
}
