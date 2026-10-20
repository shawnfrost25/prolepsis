package errlog

import (
	"encoding/json"
	"errors"
	"net/http"
	"prolepsis/internal/lib"

	"github.com/rs/zerolog"
)

func Json(logger zerolog.Logger, w http.ResponseWriter, subject string, trace string, err error) {

	respond := func(status int, errorCode string, message string) {
		lib.Pretty(w, status, lib.Error{
			Code:    errorCode,
			Message: message,
			TraceID: trace,
		})
	}

	synErr, isSyntaxError := errors.AsType[*json.SyntaxError](err)
	unmTypeErr, isUnmTypeErr := errors.AsType[*json.UnmarshalTypeError](err)
	invUnmErr, isInvUnmErr := errors.AsType[*json.InvalidUnmarshalError](err)
	marErr, isMarhsalErr := errors.AsType[*json.MarshalerError](err)
	unsupTypeErr, isUnsupTypeErr := errors.AsType[*json.UnsupportedTypeError](err)
	unsupValErr, isUnsupValErr := errors.AsType[*json.UnsupportedValueError](err)

	switch {
	case isSyntaxError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Int64("offset", synErr.Offset).
			Str("code", subject+"_payload_syntax_error").
			Msg("invalid json payload")
		respond(http.StatusBadRequest, "BAD_REQUEST", "The payload contains invalid JSON syntax")
	case isUnmTypeErr:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("field", unmTypeErr.Field).
			Str("given_type", unmTypeErr.Value).
			Str("expected_type", unmTypeErr.Type.String()).
			Int64("offset", unmTypeErr.Offset).
			Str("code", subject+"_payload_type_error").
			Msg("invalid json type")
		respond(http.StatusBadRequest, "BAD_REQUEST", "The payload contains a value with an invalid type")
	case isInvUnmErr:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("target_type", invUnmErr.Type.String()).
			Str("code", subject+"_invalid_unmarshal_target").
			Msg("invalid json unmarshal target")
		InternalRes(w, trace)
	case isMarhsalErr:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("target_type", marErr.Type.String()).
			Str("code", subject+"_marshal_failed").
			Msg("json marshaler failed")
		InternalRes(w, trace)
	case isUnsupTypeErr:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("target_type", unsupTypeErr.Type.String()).
			Str("code", subject+"_unsupported_type").
			Msg("unsupported json type")
		InternalRes(w, trace)
	case isUnsupValErr:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("value", unsupValErr.Value.String()).
			Str("description", unsupValErr.Str).
			Str("code", subject+"_unsupported_value").
			Msg("unsupported json value")
		InternalRes(w, trace)
	default:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_failed").
			Msg("json operation failed")
		InternalRes(w, trace)
	}
}
