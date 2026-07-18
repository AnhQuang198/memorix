package httpx

import "testing"

func TestCursor_RoundTrip(t *testing.T) {
	c := Cursor{SortKey: "2026-07-07T10:00:00Z", ID: "abc-123"}
	enc := c.Encode()
	if enc == "" {
		t.Fatal("encode empty")
	}
	got, err := DecodeCursor(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.SortKey != c.SortKey || got.ID != c.ID {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, c)
	}
}

func TestCursor_DecodeInvalid(t *testing.T) {
	if _, err := DecodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("expected error on invalid cursor")
	}
}

func TestCursor_EmptyDecodesToZero(t *testing.T) {
	got, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("empty cursor should be valid start: %v", err)
	}
	if got.ID != "" {
		t.Errorf("empty cursor should be zero value, got %+v", got)
	}
}
