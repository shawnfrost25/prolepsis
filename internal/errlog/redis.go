package errlog

import (
	"net/http"

	"github.com/rs/zerolog"
)

func RedisAuthenticationError(logger zerolog.Logger, w http.ResponseWriter, trace string, err error) {
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "redis_authentication_failed").
		Msg("failed redis authentication")
	Internal(w, trace)
}
