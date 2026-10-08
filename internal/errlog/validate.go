package errlog

import (
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func EmptyValueError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Int("status", http.StatusBadRequest).
		Str("code", "missing_"+subject).
		Msg(subjectMsg + " is missing")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Missing parameter",
		Details: map[string]string{
			"reason": "a specific value was not provided",
		},
		TraceID: trace,
	})
}

func EmptyContextError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Int("status", http.StatusInternalServerError).
		Str("code", "missing_"+subject+"_context").
		Msg(subjectMsg + " context is missing")
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Missing context",
		Details: map[string]string{
			"reason": "a specific context was not provided",
		},
		TraceID: trace,
	})
}

func ParsingError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Err(err).
		Int("status", http.StatusBadRequest).
		Str("code", "invalid_"+subject).
		Msg(subjectMsg + " could not be parsed")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Malformed request",
		TraceID: trace,
	})
}

func UnexpectedError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "unexpected_"+subject).
		Msg(subjectMsg + " failed unexpectedly")

	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}

func UnauthorizedError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Err(err).
		Int("status", http.StatusUnauthorized).
		Str("code", "unauthorized_"+subject).
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
		Str("code", "forbidden_"+subject).
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

func ConflictError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	logger.Warn().
		Int("status", http.StatusConflict).
		Str("code", subject+"_already_exists").
		Msg("given resource already exists")
	lib.Pretty(w, http.StatusConflict, lib.Error{
		Code:    "CONFLICT",
		Message: "A resource with the provided identifier already exists",
		TraceID: trace,
	})
}
