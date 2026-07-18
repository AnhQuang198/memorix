package authmw

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Principal là chủ thể đã xác thực (AD-11) — xuống service, KHÔNG xuống domain.
type Principal struct {
	UserID string
	Role   string
	Plan   string
}

// JWTManager phát/verify access JWT HS256 ngắn hạn (15m).
type JWTManager struct {
	secret []byte
	ttl    time.Duration
	issuer string
	now    func() time.Time
}

func NewJWTManager(secret []byte, ttl time.Duration, issuer string) *JWTManager {
	return &JWTManager{secret: secret, ttl: ttl, issuer: issuer, now: time.Now}
}

func (m *JWTManager) Issue(userID, role, plan string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"plan": plan,
		"iss":  m.issuer,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	return signed, exp, err
}

func (m *JWTManager) Verify(token string) (Principal, error) {
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	_, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return Principal{}, err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Principal{}, errors.New("authmw: missing sub claim")
	}
	role, _ := claims["role"].(string)
	plan, _ := claims["plan"].(string)
	return Principal{UserID: sub, Role: role, Plan: plan}, nil
}
