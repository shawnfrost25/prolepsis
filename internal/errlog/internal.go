package errlog

import (
	"encoding/json"
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func Internal(w http.ResponseWriter, trace string) {
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}

func GenerateError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", subject+"_generation_failed").
		Msg("failed to generate " + subjectMsg)
	Internal(w, trace)
}

func InternalEmptyValueError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Int("status", http.StatusInternalServerError).
		Str("field", subject).
		Str("code", subject+"_missing").
		Msg(subjectMsg + " is empty")
	Internal(w, trace)
}

func InternalParsingError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", subject+"_malformed").
		Msg(subjectMsg + " is malformed")
	Internal(w, trace)
}

func InternalJsonSyntaxError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, synErr *json.SyntaxError) {
	logger.Error().
		Err(synErr).
		Int("status", http.StatusInternalServerError).
		Str("code", subject+"_syntax_error").
		Msg("json syntax error")
	Internal(w, trace)
}

func InternalJsonUnmarshalTypeError(logger zerolog.Logger, w http.ResponseWriter, trace string, unmTypeErr *json.UnmarshalTypeError) {
	logger.Error().
		Err(unmTypeErr).
		Int("status", http.StatusInternalServerError).
		Str("code", "structure_invalid").
		Str("field", unmTypeErr.Field).
		Str("given_type", unmTypeErr.Value).
		Str("expected_type", unmTypeErr.Type.String()).
		Msg("invalid structure")
	Internal(w, trace)
}
