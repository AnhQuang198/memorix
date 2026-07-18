package config

import "fmt"

type Config struct {
	AppEnv      string
	HTTPPort    string
	DatabaseURL string
	RedisURL    string
}

// Load đọc config theo 12-factor. getenv được inject để test.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		AppEnv:      or(getenv("APP_ENV"), "development"),
		HTTPPort:    or(getenv("HTTP_PORT"), "8080"),
		DatabaseURL: getenv("DATABASE_URL"),
		RedisURL:    getenv("REDIS_URL"),
	}
	if c.AppEnv == "production" && c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL required in production")
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
