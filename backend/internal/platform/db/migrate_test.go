package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrate_CreatesSchemas(t *testing.T) {
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("memorix"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	defer func() { _ = pg.Terminate(ctx) }()

	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	conn, _ := pgx.Connect(ctx, dsn)
	defer func() { _ = conn.Close(ctx) }()
	var count int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.schemata
		 WHERE schema_name IN ('identity','vocabulary','scheduling','review','progress','notification')`).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 6 {
		t.Errorf("expected 6 module schemas, got %d", count)
	}
}

func TestMigrate_CreatesVocabularyTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("memorix"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	defer func() { _ = pg.Terminate(ctx) }()

	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn, _ := pgx.Connect(ctx, dsn)
	defer func() { _ = conn.Close(ctx) }()

	var n int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema='vocabulary'
		 AND table_name IN ('entries','meanings','examples','pronunciations',
		                    'synonyms_antonyms','curated_decks','deck_enrollments')`).Scan(&n)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7 vocabulary tables, got %d", n)
	}

	// Generated column term_normalized bỏ dấu + lower.
	var norm string
	if err := conn.QueryRow(ctx,
		`INSERT INTO vocabulary.entries (owner_id, term) VALUES (gen_random_uuid(), 'Résumé')
		 RETURNING term_normalized`).Scan(&norm); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if norm != "resume" {
		t.Errorf("term_normalized = %q, want %q", norm, "resume")
	}
}
