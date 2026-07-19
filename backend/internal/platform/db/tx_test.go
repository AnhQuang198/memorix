package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestWithinTx_CommitsOnNil(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.tx_probe(n int)`)
	require.NoError(t, err)

	err = db.WithinTx(ctx, pool, func(q db.Querier) error {
		_, e := q.Exec(ctx, `INSERT INTO public.tx_probe(n) VALUES (1)`)
		return e
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.tx_probe`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestWithinTx_RollsBackOnError(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.tx_probe2(n int)`)
	require.NoError(t, err)

	wantErr := context.Canceled
	err = db.WithinTx(ctx, pool, func(q db.Querier) error {
		_, _ = q.Exec(ctx, `INSERT INTO public.tx_probe2(n) VALUES (1)`)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.tx_probe2`).Scan(&n))
	require.Equal(t, 0, n, "insert phải bị rollback")
}
