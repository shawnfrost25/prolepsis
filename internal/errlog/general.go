package errlog

import (
	"net/http"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func General(logger zerolog.Logger, w http.ResponseWriter, subject string, kind failure.KindType, err error, trace string) {
	respond := func(status int, errorCode string, message string) {
		lib.Pretty(w, status, lib.Error{
			Code:    errorCode,
			Message: message,
			TraceID: trace,
		})
	}

	subjectMsg := strings.ReplaceAll(subject, "_", " ")

	switch kind {
	case failure.GeneralError:
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", subject+"_failed").
			Msg(subjectMsg + " request failed")
		respond(http.StatusBadRequest, "BAD_REQUEST", "The request could not be processed")
	case failure.InternalGeneralError:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_failed").
			Msg(subjectMsg + " ended as unexpected internal error")
		InternalRes(w, trace)
	}
}
