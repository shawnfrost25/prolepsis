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

func (h *WebhookHandler) WebhookPush(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "push").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubPush
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "push_format", w, trace, err)
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

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		errlog.ParsingError(logger, "hook_id", w, trace, err)
		return
	}

	var pgErr *pgconn.PgError

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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "push_info_insert", w, trace, err)
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "push_info_insert_failed", pgErr, err)
		}
		errlog.UnexpectedError(logger, "push_info_insert", w, trace, err)
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
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "push_commit_insert", w, trace, err)
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, trace, "push_commit_insert_failed", pgErr, err)
			}
			errlog.UnexpectedError(logger, "push_commit_insert", w, trace, err)
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
