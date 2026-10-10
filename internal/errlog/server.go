package errlog

import (
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func UnexpectedError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", subject+"_failed").
		Msg(subjectMsg + " failed unexpectedly")
	Internal(w, trace)
}

func DeadlineExceededError(logger zerolog.Logger, subject string, w http.ResponseWriter, trace string, err error) {
	subjectMsg := strings.ReplaceAll(subject, "_", " ")
	logger.Error().
		Err(err).
		Int("status", http.StatusGatewayTimeout).
		Str("code", subject+"_timeout").
		Msg(subjectMsg + " timeout")
	lib.Pretty(w, http.StatusGatewayTimeout, lib.Error{
		Code:    "GATEWAY_TIMEOUT",
		Message: "Timeout hit",
		Details: map[string]string{
			"reason": "stopped due to timeout",
			"fix":    "please try again later",
		},
		TraceID: trace,
	})
}
