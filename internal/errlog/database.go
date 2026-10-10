package errlog

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

func ErrConnClosed(logger zerolog.Logger, w http.ResponseWriter, trace string, err error) {
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "database_connection_closed").
		Msg("database connection closed")
	Internal(w, trace)
}

func PgConnError(logger zerolog.Logger, w http.ResponseWriter, trace string, code string, pgErr *pgconn.PgError, err error) {
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", code).
		Str("sqlstate", pgErr.Code).
		Msg("database operation failed")
	Internal(w, trace)
}
