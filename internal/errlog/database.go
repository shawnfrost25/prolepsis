package errlog

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/lib"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

func Database(logger zerolog.Logger, w http.ResponseWriter, subject string, trace string, err error) {
	respond := func(status int, errorCode string, message string) {
		lib.Pretty(w, status, lib.Error{
			Code:    errorCode,
			Message: message,
			TraceID: trace,
		})
	}

	pgErr, isPgErr := errors.AsType[*pgconn.PgError](err)

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		logger.Error().
			Err(err).
			Int("status", http.StatusGatewayTimeout).
			Str("code", "database_timeout").
			Msg("database operation timed out")
		respond(http.StatusGatewayTimeout, "TIMEOUT", "The operation timed out")
	case errors.Is(err, pgconn.ErrConnClosed):
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "database_connection_closed").
			Msg("database connection closed")
		InternalRes(w, trace)
	case errors.Is(err, context.Canceled):
		logger.Warn().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "database_operation_canceled").
			Msg("database operation canceled")
		InternalRes(w, trace)
	case errors.Is(err, pgx.ErrNoRows):
		logger.Warn().
			Err(err).
			Int("status", http.StatusNotFound).
			Str("code", subject+"_not_found").
			Msg("nothing matches the provided resource")
		respond(http.StatusNotFound, "NOT_FOUND", "Nothing matches the provided resource")
	case isPgErr:
		switch pgErr.SQLState() {
		case "23505":
			logger.Warn().
				Err(err).
				Int("status", http.StatusConflict).
				Str("sqlstate", pgErr.SQLState()).
				Str("constraint", pgErr.ConstraintName).
				Str("schema", pgErr.SchemaName).
				Str("table", pgErr.TableName).
				Str("code", subject+"_already_exists").
				Msg("insert conflict")
			respond(http.StatusConflict, "CONFLICT", "A resource with the provided identifier already exists")
		case "23503":
			logger.Warn().
				Err(err).
				Int("status", http.StatusConflict).
				Str("sqlstate", pgErr.SQLState()).
				Str("constraint", pgErr.ConstraintName).
				Str("schema", pgErr.SchemaName).
				Str("table", pgErr.TableName).
				Str("code", subject+"_foreign_key_violation").
				Msg("resource relationship conflict")
			respond(http.StatusConflict, "CONFLICT", "The operation conflicts with an existing resource relationship")
		case "23502":
			logger.Warn().
				Err(err).
				Int("status", http.StatusBadRequest).
				Str("sqlstate", pgErr.SQLState()).
				Str("schema", pgErr.SchemaName).
				Str("table", pgErr.TableName).
				Str("column", pgErr.ColumnName).
				Str("code", subject+"_missing").
				Msg("required field missing")
			respond(http.StatusBadRequest, "BAD_REQUEST", "A required field is missing")
		case "22P02":
			logger.Warn().
				Err(err).
				Int("status", http.StatusBadRequest).
				Str("sqlstate", "22P02").
				Str("code", subject+"_invalid").
				Msg("invalid value")
			respond(http.StatusBadRequest, "BAD_REQUEST", "A provided value is invalid")
		case "40P01":
			logger.Error().
				Err(err).
				Int("status", http.StatusServiceUnavailable).
				Str("sqlstate", "40P01").
				Str("code", subject+"_deadlock").
				Msg("database deadlock detected")
			respond(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "The operation could not be completed temporarily")
		case "40001":
			logger.Error().
				Err(err).
				Int("status", http.StatusServiceUnavailable).
				Str("sqlstate", "40001").
				Str("code", subject+"_serialization_failure").
				Msg("database serialization failure")
			respond(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "The operation could not be completed temporarily")
		case "25P02":
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("sqlstate", "25P02").
				Str("code", subject+"_transaction_failed").
				Msg("database transaction already failed")
			InternalRes(w, trace)
		case "53300":
			logger.Error().
				Err(err).
				Int("status", http.StatusServiceUnavailable).
				Str("sqlstate", "53300").
				Str("code", "database_connections_exhausted").
				Msg("database connection limit reached")
			respond(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "The service is temporarily unavailable")
		case "57014":
			logger.Warn().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("sqlstate", "57014").
				Str("code", subject+"_query_canceled").
				Msg("database query canceled")
			InternalRes(w, trace)
		default:
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("sqlstate", pgErr.SQLState()).
				Str("schema", pgErr.SchemaName).
				Str("table", pgErr.TableName).
				Str("routine", pgErr.Routine).
				Str("code", subject+"_failed").
				Msg("unexpected database error")
			InternalRes(w, trace)
		}
	default:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_failed").
			Msg("unexpected database error")
		InternalRes(w, trace)
	}
}
