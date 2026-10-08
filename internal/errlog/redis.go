package errlog

import (
	"net/http"
	"prolepsis/internal/lib"

	"github.com/rs/zerolog"
)

func RedisNilError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	logger.Warn().
		Err(err).
		Int("status", http.StatusNotFound).
		Str("code", subject+"_not_found").
		Msg("missing or expired redis key")
	lib.Pretty(w, http.StatusNotFound, lib.Error{
		Code:    "NOT_FOUND",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}

func RedisAuthenticationError(logger zerolog.Logger, w http.ResponseWriter, trace string, err error) {
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "redis_authentication_failed").
		Msg("failed redis authentication")
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}
