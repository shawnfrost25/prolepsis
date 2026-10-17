package webhook

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h *WebhookHandler) WebhookIssueComment(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issue_comment").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req IssueCommentsWebhook
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "issue_comment_format", w, trace, err)
		return
	}

	var pgErr *pgconn.PgError

	insertedRows, err := h.Queries.InsertWebhookDeliveries(timeout, deliveryID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "delivery_id_existence_check", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "delivery_id_existence_check_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "delivery_id_existence_check", w, trace, err)
		return
	}
	if insertedRows == 0 {
		logger.Debug().
			Int("status", http.StatusOK).
			Str("code", "delivery_id_deduplication").
			Msg("delivery id already exists")
		lib.Pretty(w, http.StatusOK, nil)
	}

	var authorLogin *string
	if req.IssueComment.Author != nil {
		authorLogin = &req.IssueComment.Author.Login

		if req.IssueComment.PullRequest != nil {
			exists, err := h.Queries.ExistPullRequest(timeout, req.Issue.ID)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errlog.DeadlineExceededError(logger, "pull_request_existence_check", w, trace, err)
					return
				}
				if errors.Is(err, pgconn.ErrConnClosed) {
					errlog.ErrConnClosed(logger, w, trace, err)
					return
				}
				if errors.As(err, &pgErr) {
					errlog.PgConnError(logger, w, trace, "pull_request_existence_check_failed", pgErr, err)
					return
				}
				errlog.UnexpectedError(logger, "pull_request_existence_check", w, trace, err)
				return
			}

			if !exists {
				errlog.NotFoundError(logger, "issue", w, trace, err)
				return
			}

			err = h.Queries.InsertPullRequestComment(timeout, db.InsertPullRequestCommentParams{
				ID:          req.IssueComment.ID,
				PrID:        req.Issue.ID,
				Action:      &req.Action,
				AuthorLogin: authorLogin,
				Body:        req.IssueComment.Body,
				IsMinimized: *req.IssueComment.IsMinimized,
				HtmlUrl:     req.IssueComment.HtmlUrl,
				CreatedAt:   pgtype.Timestamptz{Time: req.IssueComment.CreatedAt, Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: req.IssueComment.UpdatedAt, Valid: true},
			})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errlog.DeadlineExceededError(logger, "pull_request_comment_insert", w, trace, err)
					return
				}
				if errors.Is(err, pgconn.ErrConnClosed) {
					errlog.ErrConnClosed(logger, w, trace, err)
					return
				}
				if errors.As(err, &pgErr) {
					errlog.PgConnError(logger, w, trace, "pull_request_comment_insert_failed", pgErr, err)
					return
				}
				errlog.UnexpectedError(logger, "pull_request_comment_insert", w, trace, err)
				return
			}

			logger.Info().
				Str("code", "success").
				Msg("successfully inserted pull request comment")
			lib.Pretty(w, http.StatusOK, nil)
			return
		}

		exists, err := h.Queries.ExistIssue(timeout, req.Issue.ID)
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

		err = h.Queries.InsertIssueComment(timeout, db.InsertIssueCommentParams{
			ID:          req.IssueComment.ID,
			IssueID:     req.Issue.ID,
			Action:      &req.Action,
			AuthorLogin: authorLogin,
			Body:        req.IssueComment.Body,
			IsMinimized: *req.IssueComment.IsMinimized,
			HtmlUrl:     req.IssueComment.HtmlUrl,
			CreatedAt:   pgtype.Timestamptz{Time: req.IssueComment.CreatedAt, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: req.IssueComment.UpdatedAt, Valid: true},
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
}
