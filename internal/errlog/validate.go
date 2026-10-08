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
