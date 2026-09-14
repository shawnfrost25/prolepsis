package kitanai

import (
	db "prolepsis/internal/db/sqlc"
	filegrpc "prolepsis/internal/file_grpc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
)

type Mailer struct {
	ApiKey string
}

type Handler struct {
	Pool        *pgxpool.Pool
	Queries     *db.Queries
	Mailer      *Mailer
	PdfClient   *filegrpc.PdfClient
	RedisClient *redis.Client
	RiverClient *river.Client[pgx.Tx]
}

func New(pool *pgxpool.Pool, queries *db.Queries, mailer *Mailer, pdfclient *filegrpc.PdfClient, rclient *redis.Client, riverclient *river.Client[pgx.Tx]) *Handler {
	return &Handler{
		Pool:        pool,
		Queries:     queries,
		Mailer:      mailer,
		PdfClient:   pdfclient,
		RedisClient: rclient,
		RiverClient: riverclient,
	}
}
