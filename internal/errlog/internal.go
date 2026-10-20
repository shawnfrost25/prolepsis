package errlog

import (
	"net/http"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func InternalRes(w http.ResponseWriter, trace string) {
	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "An internal error occurred",
		TraceID: trace,
	})
}

func Internal(logger zerolog.Logger, w http.ResponseWriter, subject string, kind failure.KindType, err error, trace string) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")

	switch kind {
	case failure.InternalEmptyValueError:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_missing").
			Msg(subjectMsg + " is missing")
		InternalRes(w, trace)
	case failure.GenerationError:
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", subject+"_generation_failed").
			Msg("failed to generate " + subjectMsg)
		InternalRes(w, trace)
	}
}
