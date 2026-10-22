package oauthGithub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"time"

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
		err = failure.Wrap("github.http.oauth.state.generate", err)
		errlog.Internal(logger, w, "oauth_state", failure.GenerationError, err, u.Trace)
		return
	}
	state := hex.EncodeToString(n)

	_, err = h.RedisClient.Set(timeout, "oauth:github:state:"+state, u.ID.String(), 10*time.Minute).Result()
	if err != nil {
		err = failure.Wrap("github.redis.oauth.state.insert", err)
		errlog.Redis(logger, w, "oauth_state_insert", err, u.Trace)
		return
	}

	url := h.GH_Config.AuthCodeURL(state)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
