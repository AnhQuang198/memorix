package domain

import "testing"

func TestPasswordStrongEnough(t *testing.T) {
	weak := []string{
		"short7A",    // < 8
		"password",   // blocklist
		"aaaaaaaa",   // 8, một lớp ký tự
		"12345678",   // blocklist
	}
	for _, pw := range weak {
		if PasswordStrongEnough(pw) {
			t.Errorf("expected %q weak (score<2)", pw)
		}
	}
	strong := []string{
		"Tr0ub4dour",         // 10, 3 lớp
		"MyPa55word!!",       // 12, 4 lớp
		"correct-horse9Batt", // dài + đa dạng
	}
	for _, pw := range strong {
		if !PasswordStrongEnough(pw) {
			t.Errorf("expected %q strong (score>=2)", pw)
		}
	}
}
