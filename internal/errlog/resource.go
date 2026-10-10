package errlog

import (
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func NotFoundError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Warn().
		Err(err).
		Int("status", http.StatusNotFound).
		Str("code", subject+"_not_found").
		Msg(subjectMsg + " not found")
	lib.Pretty(w, http.StatusNotFound, lib.Error{
		Code:    "NOT_FOUND",
		Message: "Nothing matches the provided resource",
		TraceID: trace,
	})
}

func ConflictError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string) {
	logger.Warn().
		Int("status", http.StatusConflict).
		Str("code", subject+"_already_exists").
		Msg("resource conflict")
	lib.Pretty(w, http.StatusConflict, lib.Error{
		Code:    "CONFLICT",
		Message: "A resource with the provided identifier already exists",
		TraceID: trace,
	})
}
