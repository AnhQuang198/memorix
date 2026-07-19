package service

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
)

// poolOrNil trả *pgxpool.Pool như db.Querier, hoặc nil khi pool nil (unit test).
func poolOrNil(p *pgxpool.Pool) db.Querier {
	if p == nil {
		return nil
	}
	return p
}
