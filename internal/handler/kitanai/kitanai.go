package kitanai

import (
	db "prolepsis/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Mailer struct {
	ApiKey string
}

type Handler struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Mailer  *Mailer
}

func New(pool *pgxpool.Pool, queries *db.Queries, mailer *Mailer) *Handler {
	return &Handler{
		Pool:    pool,
		Queries: queries,
		Mailer:  mailer,
	}
}
