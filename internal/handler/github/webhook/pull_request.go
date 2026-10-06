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

func (h *WebhookHandler) WebhookPullRequest(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "pull_request").Logger()
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

	var req GitHubPullRequest
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_to_unmarshal").
			Msg("failed to unmarshal the given body")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrevognized error while trying to unmarshal the given body",
			TraceID: trace,
		})
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)
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

	if req.PRInfo.State != "open" && req.PRInfo.State != "closed" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_issue_state").
			Str("given_state", req.PRInfo.State).
			Msg("the given payload's state is not 'closed' or 'open'")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, state is malformed",
			TraceID: trace,
		})
		return
	}

	auth := req.PRInfo.Auth

	var authID *int64
	if auth != nil {
		authID = &auth.ID
	}

	var authName *string
	if auth != nil {
		authName = &auth.Login
	}

	var authType *string
	if auth != nil {
		authType = &auth.Type
	}

	if req.PRInfo.State != "closed" && req.PRInfo.State != "open" {
		logger.Error().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_pull_request_state").
			Str("given_state", req.PRInfo.State).
			Msg("the given payload's state is not 'closed' or 'open'")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "Invalid payload, state is malformed",
			TraceID: trace,
		})
		return
	}

	err = h.Queries.InsertGitHubPullRequestInfo(timeout, db.InsertGitHubPullRequestInfoParams{
		ID:         req.PRInfo.ID,
		Number:     int32(req.PRInfo.Number),
		Title:      req.PRInfo.Title,
		State:      req.PRInfo.State,
		IsDraft:    req.PRInfo.IsDraft,
		IsMerged:   req.PRInfo.IsMerged,
		AuthorID:   authID,
		AuthorName: authName,
		AuthorType: authType,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the info about the pull request inside the database due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the pull request info inside the database",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to insert the pull request info inside the database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to insert the pull request info due to unrecognized error",
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

	var closedAt pgtype.Timestamptz
	if req.TimeStamp.ClosedAt != nil {
		closedAt = pgtype.Timestamptz{Time: *req.TimeStamp.ClosedAt, Valid: true}
	}

	var mergedAt pgtype.Timestamptz
	if req.TimeStamp.MergedAt != nil {
		mergedAt = pgtype.Timestamptz{Time: *req.TimeStamp.MergedAt, Valid: true}
	}

	err = h.Queries.InsertGitHubPullRequest(timeout, db.InsertGitHubPullRequestParams{
		GithubRepoID:         req.RepoInfo.ID,
		GithubUserID:         req.RepoInfo.OwnerID.ID,
		HookID:               hookID,
		EventAction:          req.EventAction,
		FullName:             req.RepoInfo.FullName,
		SenderID:             req.SentBy.ID,
		SenderName:           req.SentBy.Login,
		SenderType:           req.SentBy.Type,
		PullRequestID:        req.PRInfo.ID,
		CreatedAt:            pgtype.Timestamptz{Time: req.TimeStamp.CreatedAt, Valid: true},
		UpdatedAt:            pgtype.Timestamptz{Time: req.TimeStamp.UpdatedAt, Valid: true},
		ClosedAt:             closedAt,
		MergedAt:             mergedAt,
		Additions:            int32(req.Metrics.Additions),
		Deletions:            int32(req.Metrics.Deletions),
		ChangedFiles:         int32(req.Metrics.ChangedFiles),
		CommitsCount:         int32(req.Metrics.CommitsCount),
		CommentsCount:        int32(req.Metrics.CommentsCount),
		ReviewCommentsCount:  int32(req.Metrics.ReviewCommentsCount),
		HeadBranch:           req.GitInfo.HeadBranch,
		HeadSha:              req.GitInfo.HeadBranch,
		BaseBranch:           req.GitInfo.BaseBranch,
		MergeCommitSha:       req.GitInfo.MergeCommitSha,
		AssigneeIds:          req.Workflow.AssigneeIDs,
		RequestedReviewerIds: req.Workflow.RequestedReviewerIDs,
		RequestedTeamIds:     req.Workflow.RequestedTeamIDs,
		Labels:               req.Workflow.Labels,
		MilestoneID:          req.Workflow.MilestoneID,
	})
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("failed to insert the whole pull request inside the database due to timeout")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timedout while trying to insert the whole pull request inside the database",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert the whole pull request inside the database due to an unrecognized error")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert the whole pull request",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully managed to insert the webhook push info and the commits too")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
