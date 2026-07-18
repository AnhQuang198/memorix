package domain

import "testing"

func TestCardStatusValid(t *testing.T) {
	for _, s := range []CardStatus{StatusNew, StatusLearning, StatusReview, StatusRelearning, StatusSuspended} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if CardStatus("done").Valid() {
		t.Error("unknown status must be invalid")
	}
}

func TestDirectionValid(t *testing.T) {
	if !DirectionFrontBack.Valid() || Direction("x").Valid() {
		t.Error("direction validity wrong")
	}
}
