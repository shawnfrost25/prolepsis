package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func (h WebhookHandler) WebhookReleases(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "releases").Logger()
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
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubReleases
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_to_unmarshal").
			Msg("failed to unmarshal the given body request")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to unmarshal the given body request",
			TraceID: trace,
		})
		return
	}

	repoIDStr := strconv.FormatInt(req.Repository.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to fetch the webhook id from redis due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to get the webhook if",
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, redis.Nil) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "missing_webhook_id").
				Int64("repo_id", req.Repository.ID).
				Str("full_name", req.Repository.FullName).
				Msg("missing webhook id inside redis")
			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "Failed due to missing webhook id",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to check for the webhook id inside redis")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed due to unrecognized error while trying to check for the webhook existence inside redis",
			TraceID: trace,
		})
		return
	}

	if hookIDStr == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "empty_webhook_id").
			Msg("failed due to empty webhook id")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Empty webhook id, cannot continue",
			TraceID: trace,
		})
		return
	}

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "failed_hook_id_parsing").
			Msg("failed to parse the webhook id from string to int64")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Failed webhook id parsing",
			TraceID: trace,
		})
		return
	}

	var publishedAt pgtype.Timestamptz
	if req.Release.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: *req.Release.PublishedAt, Valid: true}
	}

	err = h.Queries.InsertGitHubReleases(timeout, db.InsertGitHubReleasesParams{
		GithubRepoID:    req.Repository.ID,
		GithubUserID:    req.Repository.OwnerID.ID,
		ReleasesID:      req.Release.ID,
		HookID:          hookID,
		Action:          req.Action,
		TagName:         req.Release.TagName,
		Name:            req.Release.Name,
		Body:            req.Release.Body,
		TargetCommitish: req.Release.TargetCommitish,
		Draft:           req.Release.Draft,
		Prerelease:      req.Release.Prerelease,
		PublishedAt:     publishedAt,
		SenderID:        req.Sender.ID,
		SenderName:      req.Sender.Login,
		SenderType:      req.Sender.Type,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the given release due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying insert the given release inside the database",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the given release due to an unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to insert the given release due to an unrecognized error",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Str("cause", "success").
		Msg("successfully inserted the given release inside the database")

	lib.Pretty(w, http.StatusOK, nil)
}
