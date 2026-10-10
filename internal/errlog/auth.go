package errlog

import (
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func UnauthorizedError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Err(err).
		Int("status", http.StatusUnauthorized).
		Str("code", subject+"_unauthorized").
		Msg("unauthorized " + subjectMsg)
	lib.Pretty(w, http.StatusUnauthorized, lib.Error{
		Code:    "UNAUTHORIZED",
		Message: "Unauthorized action",
		TraceID: trace,
	})
}

func ForbiddenError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Int("status", http.StatusForbidden).
		Str("code", subject+"_forbidden").
		Msg("not enough permissions for " + subjectMsg)
	lib.Pretty(w, http.StatusForbidden, lib.Error{
		Code:    "FORBIDDEN",
		Message: "Restricted action",
		Details: map[string]string{
			"reason": "you don't have permissions to perform the given action",
		},
		TraceID: trace,
	})
}
