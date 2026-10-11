package oauthGithub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
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
	logger := zerolog.Ctx(r.Context()).With().Str("op", "github_webhook").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	shaRaw := r.Header.Get("X-Hub-Signature-256")
	prefix := "sha256"
	sha := strings.SplitN(shaRaw, "=", 2)
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		errlog.UnexpectedError(logger, "webhook_body_read", w, trace, err)
		return
	}

	var req webhook.GitHubPush
	err = sonic.Unmarshal(bodyBytes, &req)
	if err != nil {
		errlog.ParsingError(logger, "webhook_format", w, trace, err)
		return
	}

	if len(sha) != 2 {
		errlog.InvalidError(logger, "webhook_signature", w, trace)
		return
	}

	if sha[0] != prefix || len(sha[1]) != 64 {
		var reason string
		if sha[0] != prefix && len(sha[1]) != 64 {
			reason = "fully invalid sha256"
		}
		if sha[0] != prefix && len(sha[1]) == 64 {
			reason = "prefix is not 'sha256'"
		}
		if sha[0] == prefix && len(sha[1]) != 64 {
			reason = "sha256 length is not 64"
		}
		logger = logger.With().Str("reason", reason).Logger()
		errlog.InvalidError(logger, "webhook_signature", w, trace)
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)

	secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "webhook_secret_fetch", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
			return
		}
		if errors.Is(err, redis.Nil) {
			errlog.NotFoundError(logger, "webhook_secret", w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "webhook_secret_fetch", w, trace, err)
		return
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	expected := hex.EncodeToString(mac.Sum(nil))

	n := subtle.ConstantTimeCompare([]byte(expected), []byte(sha[1]))
	if n == 0 {
		errlog.InvalidError(logger, "sha_compare", w, trace)
		return
	}

	var pgErr *pgconn.PgError

	exists, err := h.Queries.RepoExistsInsideGitHub(timeout, req.RepoInfo.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "repos_existence_check", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "repo_existence_check_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "repo_existence_check", w, trace, err)
		return
	}
	if !exists {
		logger.Warn().
			Int("status", http.StatusOK).
			Int64("repo_id", req.RepoInfo.ID).
			Str("full_name", req.RepoInfo.FullName).
			Msg("missing repository")
		lib.Pretty(w, http.StatusOK, lib.Error{
			Code:    "OK",
			Message: "Repository is not registered, ignoring push event",
			TraceID: trace,
		})
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")

	if deliveryID == "" {
		errlog.EmptyValueError(logger, "delivery_id", w, trace)
		return
	}
	logger.Info().
		Str("delivery_id", deliveryID).
		Msg("provided delivery id by the webhook")

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		errlog.InvalidError(logger, "webhook_payload", w, trace)
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
