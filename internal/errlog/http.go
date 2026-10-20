package errlog

import (
	"net/http"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func Validate(logger zerolog.Logger, w http.ResponseWriter, subject string, kind failure.KindType, err error, trace string) {
	respond := func(status int, errorCode string, message string) {
		lib.Pretty(w, status, lib.Error{
			Code:    errorCode,
			Message: message,
			TraceID: trace,
		})
	}

	subjectMsg := strings.ReplaceAll(subject, "_", " ")

	switch kind {

	case failure.EmptyValueError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", subject+"_missing").
			Msg(subjectMsg + " is missing")
		respond(http.StatusBadRequest, "BAD_REQUEST", "A required field is missing")
	case failure.MalformedError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", subject+"_malformed").
			Msg("resource is malformed")
		respond(http.StatusBadRequest, "BAD_REQUEST", "The provided data is malformed")
	case failure.InvalidError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", subject+"_invalid").
			Msg("request validation failed")
		respond(http.StatusBadRequest, "BAD_REQUEST", "The given resource does not comply with the requirements")
	case failure.UnauthorizedError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusUnauthorized).
			Str("code", subject+"_unauthorized").
			Msg("unauthorized " + subjectMsg)
		respond(http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized action")
	case failure.ForbiddenError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusForbidden).
			Str("code", subject+"_forbidden").
			Msg("not enough permissions for " + subjectMsg)
		respond(http.StatusForbidden, "FORBIDDEN", "Restricted action")
	case failure.ConflictError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusConflict).
			Str("code", subject+"_already_exists").
			Msg("resource conflict")
		respond(http.StatusConflict, "CONFLICT", "A resource with the provided identifier already exists")
	case failure.NotFoundError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusNotFound).
			Str("code", subject+"_not_found").
			Msg(subjectMsg + " not found")
		respond(http.StatusNotFound, "NOT_FOUND", "Nothing matches the provided resource")
	}
}
