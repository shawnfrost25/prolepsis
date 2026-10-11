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

func (h *WebhookHandler) WebhookIssueComment(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issue_comment").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubIssueComment
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "issue_comment_format", w, trace, err)
		return
	}

	change := req.Changes
	var bodyFrom *string
	if change != nil {
		bodyFrom = &req.Changes.Body.From
	}

	repoIDStr := strconv.FormatInt(req.Repository.ID, 10)
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

	if req.Issue.Title == "" {
		errlog.EmptyValueError(logger, "issue_title", w, trace)
		return
	}

	if req.Issue.State != "open" && req.Issue.State != "closed" {
		errlog.InvalidError(logger, "issue_state", w, trace)
		return
	}

	var pgErr *pgconn.PgError

	exists, err := h.Queries.IssueInfoExistsInsideTheGitHub(timeout, req.Issue.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "issue_existence_check", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "issue_existence_check_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "issue_existence_check", w, trace, err)
		return
	}

	if !exists {
		errlog.NotFoundError(logger, "issue", w, trace, err)
		return
	}

	positiveReactions := req.Issue.Reactions.Hooray + req.Issue.Reactions.PositiveReaction + req.Issue.Reactions.Heart + req.Issue.Reactions.Laugh + req.Issue.Reactions.Rocket
	negativeReactions := req.Issue.Reactions.Confused + req.Issue.Reactions.NegativeReaction

	err = h.Queries.InsertGitHubIssueComment(timeout, db.InsertGitHubIssueCommentParams{
		IssueID:           req.Issue.ID,
		Action:            req.Action,
		ChangeFrom:        bodyFrom,
		CreatedAt:         pgtype.Timestamptz{Time: req.Comment.CreatedAt, Valid: true},
		UpdatedAt:         pgtype.Timestamptz{Time: req.Comment.UpdatedAt, Valid: true},
		AuthorAssociation: req.Comment.AuthorAssociation,
		CommentID:         req.Comment.ID,
		CommentorID:       req.Comment.User.ID,
		CommentorName:     req.Comment.User.Name,
		CommentorType:     req.Comment.User.Type,
		Body:              req.Comment.Body,
		PositiveReactions: int32(positiveReactions),
		NegativeReactions: int32(negativeReactions),
		TotalReactions:    int32(req.Comment.Reactions.TotalCount),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "issue_comment_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "issue_comment_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "issue_comment_insert", w, trace, err)
		return
	}

	logger.Info().
		Str("code", "success").
		Msg("successfully inserted issue comment")

	lib.Pretty(w, http.StatusOK, nil)
}
