package kitanai

import (
	db "prolepsis/internal/db/sqlc"
	filegrpc "prolepsis/internal/file_grpc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Mailer struct {
	ApiKey string
}

type Handler struct {
	Pool      *pgxpool.Pool
	Queries   *db.Queries
	Mailer    *Mailer
	PdfClient *filegrpc.PdfClient
}

func New(pool *pgxpool.Pool, queries *db.Queries, mailer *Mailer, pdfclient *filegrpc.PdfClient) *Handler {
	return &Handler{
		Pool:      pool,
		Queries:   queries,
		Mailer:    mailer,
		PdfClient: pdfclient,
	}
}
