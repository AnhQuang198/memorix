package security

import "github.com/alexedwards/argon2id"

// Argon2Hasher implements identity ports.Hasher bằng argon2id (NFR-7).
type Argon2Hasher struct {
	params *argon2id.Params
}

func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{params: argon2id.DefaultParams}
}

func (h *Argon2Hasher) Hash(plain string) (string, error) {
	return argon2id.CreateHash(plain, h.params)
}

func (h *Argon2Hasher) Verify(plain, hash string) (bool, error) {
	if hash == "" {
		return false, nil
	}
	return argon2id.ComparePasswordAndHash(plain, hash)
}
