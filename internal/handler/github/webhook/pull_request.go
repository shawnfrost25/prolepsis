package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
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
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubPullRequest
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "pull_request_format", w, trace, err)
		return
	}

	repoIDStr := strconv.FormatInt(req.RepoInfo.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "hook_id_fetch", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
		}
		if errors.Is(err, redis.Nil) {
			errlog.NotFoundError(logger, "hook_id", w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "hook_id_fetch", w, trace, err)
		return
	}

	if hookIDStr == "" {
		errlog.EmptyValueError(logger, "hook_id", w, trace)
		return
	}

	if req.PRInfo.State != "open" && req.PRInfo.State != "closed" {
		errlog.InvalidError(logger, "issue_state", w, trace)
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
		errlog.InvalidError(logger, "pull_request_state", w, trace)
		return
	}

	var pgErr *pgconn.PgError

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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "pull_request_info_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "pull_request_info_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "pull_request_info_insert", w, trace, err)
		return
	}

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		errlog.ParsingError(logger, "hook_id", w, trace, err)
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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "pull_request_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "pull_request_insert", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "pull_request_insert", w, trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully managed to insert the webhook push info and the commits too")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
