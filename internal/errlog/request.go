package errlog

import (
	"net/http"
	"prolepsis/internal/lib"
	"strings"

	"github.com/rs/zerolog"
)

func DecodeError(logger zerolog.Logger, w http.ResponseWriter, trace string, err error) {
	logger.Warn().
		Err(err).
		Int("status", http.StatusBadRequest).
		Str("code", "failed_to_decode").
		Msg("request rejected")
	lib.Pretty(w, http.StatusBadRequest, lib.Error{
		Code:    "BAD_REQUEST",
		Message: "Malformed payload",
		Details: map[string]string{
			"reason": "the given payload is invalid",
			"fix":    "please, check the given information and provide a decent payload",
		},
		TraceID: trace,
	})
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
		Message: "Ended up timing out",
		Details: map[string]string{
			"reason": "stopped due to timeout",
			"fix":    "please try again later",
		},
		TraceID: trace,
	})
}
