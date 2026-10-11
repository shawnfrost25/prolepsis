package oauthGithub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func (h *GitHubHandler) GitHubRedirect(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "github_redirect").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	n := make([]byte, 16)
	_, err := rand.Read(n)
	if err != nil {
		errlog.GenerateError(logger, "bytes_read", w, u.Trace, err)
		return
	}
	state := hex.EncodeToString(n)

	_, err = h.RedisClient.Set(timeout, "oauth:github:state:"+state, u.ID.String(), 10*time.Minute).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "state_insert", w, u.Trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, u.Trace, err)
		}
		errlog.UnexpectedError(logger, "state_insert", w, u.Trace, err)
		return
	}

	url := h.GH_Config.AuthCodeURL(state)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
