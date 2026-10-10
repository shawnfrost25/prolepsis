package errlog

import (
	"encoding/json"
	"net/http"
	"prolepsis/internal/lib"

	"github.com/rs/zerolog"
)

func EmptyValueError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	logger.Warn().
		Int("status", http.StatusBadRequest).
		Str("field", subject).
		Str("code", subject+"_missing").
		Msg("request rejected")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "A required field is missing",
		TraceID: trace,
	})
}

func ParsingError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	logger.Warn().
		Err(err).
		Int("status", http.StatusBadRequest).
		Str("code", subject+"_malformed").
		Msg("request rejected")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "malformed value",
		TraceID: trace,
	})
}

func InvalidError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	logger.Warn().
		Int("status", http.StatusBadRequest).
		Str("code", subject+"_invalid").
		Msg("request validation failed")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Invalid resource",
		Details: map[string]string{
			"reason": "the given resource does not comply with the requirements",
		},
		TraceID: trace,
	})
}

func JsonSyntaxError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, synErr *json.SyntaxError) {
	logger.Warn().
		Str("error", synErr.Error()).
		Int("status", http.StatusBadRequest).
		Str("code", subject+"_syntax_error").
		Msg("request rejected")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Invalid json syntax",
		TraceID: trace,
	})
}

func JsonUnmarshalTypeError(logger zerolog.Logger, w http.ResponseWriter, trace string, unmTypeErr *json.UnmarshalTypeError) {
	logger.Warn().
		Str("error", unmTypeErr.Error()).
		Int("status", http.StatusBadRequest).
		Str("code", "structure_invalid").
		Str("field", unmTypeErr.Field).
		Str("given_type", unmTypeErr.Value).
		Str("expected_type", unmTypeErr.Type.String()).
		Msg("invalid json type")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Mismatching json type",
		TraceID: trace,
	})
}
