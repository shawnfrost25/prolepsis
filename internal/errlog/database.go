package errlog

import (
	"net/http"
	"prolepsis/internal/lib"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

func NoRowsError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	logger.Warn().
		Int("status", http.StatusNotFound).
		Str("code", subject+"_not_found").
		Msg(subject + " not found")
	lib.Pretty(w, http.StatusNotFound, lib.Error{
		Code:    "NOT_FOUND",
		Message: "Resource not found",
		TraceID: trace,
	})
}

func ErrConnClosed(logger zerolog.Logger, w http.ResponseWriter, trace string, err error) {
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "database_connection_closed").
		Msg("database connection closed")
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error ocurred",
		TraceID: trace,
	})
}

func PgConnError(logger zerolog.Logger, w http.ResponseWriter, trace string, code string, pgErr *pgconn.PgError) {
	logger.Error().
		Int("status", http.StatusInternalServerError).
		Str("code", code).
		Str("database_constraint", pgErr.ConstraintName).
		Str("sqlstate", pgErr.Code).
		Str("database_detail", pgErr.Detail).
		Str("database_hint", pgErr.Hint).
		Msg("database failed")
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}
