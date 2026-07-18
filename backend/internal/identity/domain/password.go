package domain

import (
	"strings"
	"unicode"
)

var commonPasswords = map[string]bool{
	"password": true, "12345678": true, "qwerty": true, "qwertyui": true,
	"letmein": true, "iloveyou": true, "admin123": true, "memorix": true,
}

// EstimateStrength trả 0–4 (xấp xỉ zxcvbn): độ dài + số lớp ký tự, phạt blocklist.
func EstimateStrength(pw string) int {
	if len(pw) < 8 {
		return 0
	}
	if commonPasswords[strings.ToLower(pw)] {
		return 0
	}
	var lower, upper, digit, sym bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			sym = true
		}
	}
	classes := 0
	for _, has := range []bool{lower, upper, digit, sym} {
		if has {
			classes++
		}
	}
	score := 0
	switch {
	case len(pw) >= 12:
		score += 2
	case len(pw) >= 10:
		score++
	}
	score += classes - 1
	if score < 0 {
		score = 0
	}
	if score > 4 {
		score = 4
	}
	return score
}

// PasswordStrongEnough: ngưỡng chấp nhận zxcvbn score >= 2 (Story 1.2).
func PasswordStrongEnough(pw string) bool { return EstimateStrength(pw) >= 2 }
