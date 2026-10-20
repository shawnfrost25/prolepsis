package errlog

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/lib"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func Redis(logger zerolog.Logger, w http.ResponseWriter, subject string, err error, trace string) {
	respond := func(status int, errorCode string, message string) {
		lib.Pretty(w, status, lib.Error{
			Code:    errorCode,
			Message: message,
			TraceID: trace,
		})
	}

	red, isRedisError := errors.AsType[redis.Error](err)

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		logger.Error().
			Err(err).
			Int("status", http.StatusGatewayTimeout).
			Str("code", "redis_timeout").
			Msg("redis operation timed out")
		respond(http.StatusGatewayTimeout, "TIMEOUT", "The operation timed out")
	case errors.Is(err, context.Canceled):
		logger.Warn().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "redis_operation_canceled").
			Msg("redis operation canceled")
		InternalRes(w, trace)
	case redis.IsAuthError(err):
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "redis_authentication_failed").
			Msg("failed redis authentication")
		InternalRes(w, trace)
	case errors.Is(err, redis.Nil):
		logger.Warn().
			Err(err).
			Int("status", http.StatusNotFound).
			Str("code", subject+"_not_found").
			Msg("expired or nonexistent redis entry")
		respond(http.StatusNotFound, "NOT_FOUND", "Nothing matches the provided resource")
	case isRedisError:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_failed").
			Msg("redis server rejected command: " + red.Error())
		InternalRes(w, trace)
	default:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_failed").
			Msg("unexpected redis error")
		InternalRes(w, trace)
	}
}
