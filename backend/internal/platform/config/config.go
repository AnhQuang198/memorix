package config

import (
	"fmt"
	"time"
)

type Config struct {
	AppEnv      string
	HTTPPort    string
	DatabaseURL string
	RedisURL    string

	JWTSecret  string
	JWTIssuer  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	VerifyTTL  time.Duration
	ResetTTL   time.Duration

	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
}

// Load đọc config theo 12-factor. getenv được inject để test.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		AppEnv:      or(getenv("APP_ENV"), "development"),
		HTTPPort:    or(getenv("HTTP_PORT"), "8080"),
		DatabaseURL: getenv("DATABASE_URL"),
		RedisURL:    getenv("REDIS_URL"),

		JWTSecret: or(getenv("JWT_SECRET"), "dev-insecure-secret-change-me"),
		JWTIssuer: or(getenv("JWT_ISSUER"), "memorix"),

		GoogleClientID:     getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET"),
		OAuthRedirectURL:   getenv("OAUTH_REDIRECT_URL"),
	}
	c.AccessTTL = parseDur(getenv("ACCESS_TTL"), 15*time.Minute)
	c.RefreshTTL = parseDur(getenv("REFRESH_TTL"), 30*24*time.Hour)
	c.VerifyTTL = parseDur(getenv("VERIFY_TTL"), 24*time.Hour)
	c.ResetTTL = parseDur(getenv("RESET_TTL"), time.Hour)

	if c.AppEnv == "production" {
		if c.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL required in production")
		}
		if c.JWTSecret == "dev-insecure-secret-change-me" {
			return Config{}, fmt.Errorf("JWT_SECRET required in production")
		}
	}
	return c, nil
}

func parseDur(v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
