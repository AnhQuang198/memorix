package security

import "testing"

func TestArgon2Hasher_RoundTrip(t *testing.T) {
	h := NewArgon2Hasher()
	hash, err := h.Hash("Tr0ub4dour!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "Tr0ub4dour!" || hash == "" {
		t.Fatal("hash must not be plaintext/empty")
	}
	ok, err := h.Verify("Tr0ub4dour!", hash)
	if err != nil || !ok {
		t.Errorf("verify correct password failed: ok=%v err=%v", ok, err)
	}
	bad, _ := h.Verify("wrong", hash)
	if bad {
		t.Error("verify must reject wrong password")
	}
}

func TestArgon2Hasher_EmptyHashRejects(t *testing.T) {
	h := NewArgon2Hasher()
	ok, err := h.Verify("anything", "")
	if err != nil || ok {
		t.Errorf("empty hash (oauth-only user) must reject, got ok=%v err=%v", ok, err)
	}
}

func TestArgon2Hasher_SaltedUnique(t *testing.T) {
	h := NewArgon2Hasher()
	a, _ := h.Hash("samePass123")
	b, _ := h.Hash("samePass123")
	if a == b {
		t.Error("same password must yield different hashes (random salt)")
	}
}
