package oauthGithub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/handler/github/webhook"
	"prolepsis/internal/lib"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func (h *GitHubHandler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "GitHubWebhook").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "missing_tracing_context").
			Msg("handler invoked without trace ID in context")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal server error occurred",
			Details: map[string]string{
				"reason": "request context pipeline uninitialized",
			},
		})
		return
	}
	shaRaw := r.Header.Get("X-Hub-Signature-256")
	prefix := "sha256"
	sha := strings.SplitN(shaRaw, "=", 2)
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_body_to_bytes_conversion").
			Msg("failed to convert the body to bytes due to an error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Couldn't convert the body to bytes due to ReadAll error",
			TraceID: trace,
		})
		return
	}

	var req webhook.GitHubPush
	err = sonic.Unmarshal(bodyBytes, &req)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "could_not_unmarshal").
			Msg("failed to unmarshal the given request payload")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to unmarshal the given request payload",
			TraceID: trace,
		})
		return
	}

	if len(sha) != 2 {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "malformed_sha").
			Msg(fmt.Sprintf("the given sha is not separated in a total od 2, but %d", len(sha)))
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Wrong sha length after split, refusing to continue",
			TraceID: trace,
		})
		return
	}

	if sha[0] != prefix || len(sha[1]) != 64 {
		var reason string
		if sha[0] != prefix && len(sha[1]) != 64 {
			reason = "both prefix and length mismatch the idiomatic sha256 format"
		}
		if sha[0] != prefix && len(sha[1]) == 64 {
			reason = "the prefix doesn't matches the idiomatic 'sha256'"
		}
		if sha[0] == prefix && len(sha[1]) != 64 {
			reason = "the lenght of the given secret is not 64"
		}
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_sha256_header").
			Str("reason", reason).
			Msg("the given sha256 is defected")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Failed to continue due to the malformed sha256",
			TraceID: trace,
		})
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)

	secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to get the given webhook secret due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to get the webhook secret",
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, redis.Nil) {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "missing_signature").
				Msg("failed to continue due to missing signature")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "The given signature is not present",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "redis_error").
			Msg("failed to get secret from redis")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Internal error retrieving webhook secret",
			TraceID: trace,
		})
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	expected := hex.EncodeToString(mac.Sum(nil))

	n := subtle.ConstantTimeCompare([]byte(expected), []byte(sha[1]))
	if n == 0 {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "sha_does_not_match").
			Msg("the provided sha doesn't matches with the expected sha")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The provided sha doesn't matches with the expected sha",
			TraceID: trace,
		})
		return
	}

	exists, err := h.Queries.RepoExistsInsideGitHub(timeout, req.RepoInfo.ID)
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to check about repository existence due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to check for the repository existence",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to check about repository existence due to unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to check repository existence",
			TraceID: trace,
		})
		return
	}
	if !exists {
		logger.Warn().
			Int("status", http.StatusOK).
			Int64("repo_id", req.RepoInfo.ID).
			Str("full_name", req.RepoInfo.FullName).
			Msg("ignoring webhook - repository is not registered in the database")

		lib.Pretty(w, http.StatusOK, lib.Error{
			Code:    "OK",
			Message: "Repository is not registered, ignoring push event",
			TraceID: trace,
		})
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")

	if deliveryID == "" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_delivery_header").
			Msg("the header 'X-GitHub-Delivery' is missing")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Failed due to missing delivery header",
			TraceID: trace,
		})
		return
	}
	logger.Info().
		Str("delivery_id", deliveryID).
		Msg("this is the provided id by the webhook")

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "payload_not_json").
			Msg("the given content type is not json")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given Content-Type is not application/json",
			TraceID: trace,
		})
		return
	}

	wh := webhook.New(h.Queries, h.RedisClient)

	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "push":
		wh.WebhookPush(w, r, bodyBytes, deliveryID)
	case "pull_request":
		wh.WebhookPullRequest(w, r, bodyBytes, deliveryID)
	case "issues":
		wh.WebhookIssues(w, r, bodyBytes, deliveryID)
	case "issue_comment":
		wh.WebhookIssueComment(w, r, bodyBytes, deliveryID)
	}
}
