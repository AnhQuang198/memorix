package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// TokenFactory tạo token opaque (refresh, verify, reset) và hash SHA-256 để lưu DB.
// Chỉ hash được lưu; raw gửi client 1 lần.
type TokenFactory struct{}

func (TokenFactory) New() (raw, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("security: crypto/rand failed: " + err.Error())
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashHex(raw)
}

func (TokenFactory) Hash(raw string) string { return hashHex(raw) }

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
