package oauthGithub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/handler/github/webhook"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h *GitHubHandler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "github_webhook").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("github.http.trace.context.fetch", err)
		errlog.Internal(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}
	shaRaw := r.Header.Get("X-Hub-Signature-256")
	prefix := "sha256"
	sha := strings.SplitN(shaRaw, "=", 2)
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		err = failure.Wrap("github.http.webhook.payload.read", err)
		errlog.General(logger, w, "webhook_body_read", failure.GeneralError, err, trace)
		return
	}

	var req webhook.Repository
	err = sonic.Unmarshal(bodyBytes, &req)
	if err != nil {
		err = failure.Wrap("github.json.webhook.repository.unmarshal", err)
		errlog.Json(logger, w, "webhook", trace, err)
		return
	}

	if len(sha) != 2 {
		err := failure.Invalid("webhook_signature", "Signature doesn't start with 'sha256=token'")
		err = failure.Wrap("github.http.webhook.signature.length_check", err)
		errlog.Http(logger, w, "webhook_signature", failure.InvalidError, err, trace)
		return
	}

	if sha[0] != prefix || len(sha[1]) != 64 {
		var reason string
		if sha[0] != prefix && len(sha[1]) != 64 {
			reason = "Fully invalid sha256"
		}
		if sha[0] != prefix && len(sha[1]) == 64 {
			reason = "Prefix is not 'sha256'"
		}
		if sha[0] == prefix && len(sha[1]) != 64 {
			reason = "Sha256 length is not 64"
		}
		err := failure.Invalid("webhook_signature", reason)
		err = failure.Wrap("github.http.webhook.signature.validate", err)
		errlog.Http(logger, w, "webhook_signature", failure.InvalidError, err, trace)
		return
	}

	repoIDStr := strconv.FormatInt(req.ID, 10)

	secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
	if err != nil {
		err = failure.Wrap("github.redis.webhook.secret.fetch", err)
		errlog.Redis(logger, w, "webhook_secret_fetch", err, trace)
		return
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	expected := hex.EncodeToString(mac.Sum(nil))

	n := subtle.ConstantTimeCompare([]byte(expected), []byte(sha[1]))
	if n == 0 {
		err := failure.Unauthorized("webhook_signature", "The provided secret doesn't match")
		err = failure.Wrap("github.http.webhook.secret.compare", err)
		errlog.Http(logger, w, "webhook_secret", failure.UnauthorizedError, err, trace)
		return
	}

	exists, err := h.Queries.ExistGitHubRepo(timeout, req.ID)
	if err != nil {
		err = failure.Wrap("github.database.repository.existence_check", err)
		errlog.Database(logger, w, "repository_existence_check", trace, err)
		return
	}
	if !exists {
		err := failure.NotFound("repository")
		err = failure.Wrap("github.database.repository.existence_check", err)
		errlog.Http(logger, w, "repository", failure.NotFoundError, err, trace)
		return
	}

	deliveryIDStr := r.Header.Get("X-GitHub-Delivery")

	if deliveryIDStr == "" {
		err := failure.EmptyValue("webhook_delivery_id")
		err = failure.Wrap("github.http.delivery.id.existence_check", err)
		errlog.Http(logger, w, "webhook_delivery_id", failure.EmptyValueError, err, trace)
		return
	}

	var deliveryID pgtype.UUID
	if err := deliveryID.Scan(deliveryIDStr); err != nil {
		err = failure.Wrap("github.http.delivery.id.convert", err)
		errlog.Http(logger, w, "webhook_delivery_id", failure.MalformedError, err, trace)
		return
	}

	logger.Info().
		Str("delivery_id", deliveryIDStr).
		Msg("provided delivery id by the webhook")

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := failure.Invalid("content_type", "Not a json payload")
		err = failure.Wrap("github.http.webhook.content.type_check", err)
		errlog.Http(logger, w, "webhook_content_type", failure.InvalidError, err, trace)
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
	case "release":
		wh.WebhookRelease(w, r, bodyBytes, deliveryID)
	case "repository":
		wh.WebhookRepository(w, r, bodyBytes, deliveryID)
	}
}
