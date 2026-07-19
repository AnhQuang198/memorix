package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier là tập method chung của *pgxpool.Pool và pgx.Tx. Repo adapter nhận
// Querier để cùng một code chạy được trong TX (unit-of-work) hoặc ngoài TX
// (read thuần) — nền cho grade nguyên tử span nhiều schema (AD-3).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
