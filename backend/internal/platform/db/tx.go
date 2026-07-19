package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithinTx chạy fn trong 1 transaction: commit nếu fn trả nil, rollback nếu lỗi.
// pgx.Tx thỏa Querier nên fn dùng chung code với path ngoài-TX.
func WithinTx(ctx context.Context, pool *pgxpool.Pool, fn func(Querier) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
