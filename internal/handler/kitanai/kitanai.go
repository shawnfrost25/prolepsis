package kitanai

import (
	db "prolepsis/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Mailer struct {
	FromEmail   string
	AppPassword string
}

type Handler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	mailer  *Mailer
}

func New(pool *pgxpool.Pool, queries *db.Queries, maier *Mailer) *Handler {
	return &Handler{
		pool:    pool,
		queries: queries,
		mailer:  maier,
	}
}
