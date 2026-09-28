package oauthGithub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/rs/zerolog"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/lib"
	"time"
)

func (h *GitHubHandler) GitHubRedirect(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "GitHubRedirect").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	n := make([]byte, 16)
	_, err := rand.Read(n)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert cryptographically secure bytes due to an urecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to Read due to an unrecognized error",
		})
		return
	}
	state := hex.EncodeToString(n)

	_, err = h.RedisClient.Set(timeout, "oauth:github:state:"+state, u.ID.String(), 10*time.Minute).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("could not create state due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to create state",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_state_creation").
			Msg("could not create state due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Could not cretae state",
			TraceID: u.Trace,
		})
		return
	}

	url := h.GH_Config.AuthCodeURL(state)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
