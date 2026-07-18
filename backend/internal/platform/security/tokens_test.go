package security

import "testing"

func TestTokenFactory_NewAndHash(t *testing.T) {
	f := TokenFactory{}
	raw, hash := f.New()
	if raw == "" || hash == "" {
		t.Fatal("New must return non-empty raw and hash")
	}
	if raw == hash {
		t.Error("raw must differ from its hash")
	}
	if got := f.Hash(raw); got != hash {
		t.Errorf("Hash(raw) not stable: %q vs %q", got, hash)
	}
}

func TestTokenFactory_Unpredictable(t *testing.T) {
	f := TokenFactory{}
	r1, _ := f.New()
	r2, _ := f.New()
	if r1 == r2 {
		t.Error("two tokens must differ")
	}
}
