package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	c, err := Load(func(k string) string { return "" })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.HTTPPort != "8080" {
		t.Errorf("default HTTPPort = %q, want 8080", c.HTTPPort)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	env := map[string]string{
		"HTTP_PORT":    "9000",
		"DATABASE_URL": "postgres://x",
		"REDIS_URL":    "redis://y",
	}
	c, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.HTTPPort != "9000" || c.DatabaseURL != "postgres://x" || c.RedisURL != "redis://y" {
		t.Errorf("config not read from env: %+v", c)
	}
}

func TestLoad_MissingRequiredInProd(t *testing.T) {
	env := map[string]string{"APP_ENV": "production"}
	if _, err := Load(func(k string) string { return env[k] }); err == nil {
		t.Error("expected error: DATABASE_URL required in production")
	}
}
