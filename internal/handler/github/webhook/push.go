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

func (h *WebhookHandler) WebhookPush(w http.ResponseWriter, r *http.Request, body []byte) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "push").Logger()
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

	var req GitHubPush
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "could_not_decode").
			Msg("failed to decode the given request payload")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to decode the given request payload",
			TraceID: trace,
		})
		return
	}

	var headCommitSHA *string
	if req.HeadCommitID != nil {
		headCommitSHA = &req.HeadCommitID.SHA
	} else {
		headCommitSHA = nil
	}

	const zeroSHA = "0000000000000000000000000000000000000000"

	isZero := func(sha *string) *string {
		if sha == nil || *sha == "" || *sha == zeroSHA {
			return nil
		}
		return sha
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to fetc the webhook id from redis due to timeout")
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
				Int64("repo_id", req.RepoInfo.ID).
				Str("full_name", req.RepoInfo.FullName).
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

	pushID, err := h.Queries.InsertGitHubPushInfo(timeout, db.InsertGitHubPushInfoParams{
		GithubRepoID: req.RepoInfo.ID,
		GithubUserID: req.RepoInfo.OwnerID.ID,
		HookID:       hookID,
		FullName:     req.RepoInfo.FullName,
		Ref:          req.Ref,
		BeforeSha:    isZero(&req.BeforeSHA),
		AfterSha:     isZero(&req.AfterSHA),
		HeadCommitID: headCommitSHA,
		Compare:      req.Compare,
		Forced:       req.Forced,
		Created:      req.Created,
		Deleted:      req.Deleted,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timeout while trying to insert the github webhook push information")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Failed to insert the webhook push information",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to insert the github webhook push information")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to insert the github webhook push information",
			TraceID: trace,
		})
		return
	}

	for _, commit := range req.CommitInfo {
		err := h.Queries.InsertGitHubPushCommitInfo(timeout, db.InsertGitHubPushCommitInfoParams{
			PushID:      pushID,
			CommitSha:   commit.CommitSha,
			Message:     commit.Message,
			Added:       commit.Added,
			Removed:     commit.Removed,
			Modified:    commit.Modified,
			Url:         commit.URL,
			CommittedAt: pgtype.Timestamptz{Time: commit.CommittedAt, Valid: true},
		})
		if err != nil {
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Str("commit_sha", commit.CommitSha).
					Str("push_id", pushID.String()).
					Str("full_name", req.RepoInfo.FullName).
					Msg("Failed to insert the commit info, due to timeout")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timedout while trying to insert the commit info inside the database",
					TraceID: trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Str("commit_sha", commit.CommitSha).
				Str("push_id", pushID.String()).
				Msg("hit an urecognized error while trying to insert the github webhook info inside the database")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error while trying to insert the github webhook info inside the database",
				TraceID: trace,
			})
			return
		}

		logger.Debug().
			Str("commit_sha", commit.CommitSha).
			Str("push_id", pushID.String()).
			Str("full_name", req.RepoInfo.FullName).
			Msg("successfully inserted commit")
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully managed to insert the webhook push info and the commits too")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
