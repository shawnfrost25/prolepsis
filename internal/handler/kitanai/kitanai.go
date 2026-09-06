package kitanai

import (
	db "prolepsis/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Created a Handler to don't hit
type Handler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func New(pool *pgxpool.Pool, queries *db.Queries) *Handler {
	return &Handler{
		pool:    pool,
		queries: queries,
	}
}
