package service

import (
	"context"
	"time"
)

// fakeHasher: hash tất định "h:"+plain (nhanh, xác định).
type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (fakeHasher) Verify(p, h string) (bool, error) {
	return h != "" && h == "h:"+p, nil
}

// fakeSecrets: raw đếm tăng, hash = "H("+raw+")".
type fakeSecrets struct{ n int }

func (f *fakeSecrets) New() (string, string) {
	f.n++
	raw := "tok-" + itoa(f.n)
	return raw, "H(" + raw + ")"
}
func (f *fakeSecrets) Hash(raw string) string { return "H(" + raw + ")" }

// fakeIssuer: access = "jwt:"+userID.
type fakeIssuer struct{ now func() time.Time }

func (f fakeIssuer) Issue(userID, _, _ string) (string, time.Time, error) {
	return "jwt:" + userID, f.now().Add(15 * time.Minute), nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

// fakeLimiter: allow theo cờ; đếm Reset.
type fakeLimiter struct {
	allow  bool
	resets int
}

func (l *fakeLimiter) Allow(context.Context, string) (bool, error) { return l.allow, nil }
func (l *fakeLimiter) Reset(context.Context, string)               { l.resets++ }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
