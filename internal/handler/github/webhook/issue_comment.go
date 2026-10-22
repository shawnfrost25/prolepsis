package webhook

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h *WebhookHandler) WebhookIssueComment(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issue_comment").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("github.http.trace.context.fetch", err)
		errlog.Internal(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req IssueCommentsWebhook
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		err = failure.Wrap("github.json.issue_comment.payload.unmarshal", err)
		errlog.Json(logger, w, "issue_comment", trace, err)
		return
	}

	//

	insertedRows, err := h.Queries.InsertWebhookDeliveries(timeout, deliveryID)
	if err != nil {
		err = failure.Wrap("github.database.delivery.id.insert", err)
		errlog.Database(logger, w, "webhook_delivery_id_insert", trace, err)
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
				err = failure.Wrap("github.database.pull_request.existence_check", err)
				errlog.Database(logger, w, "pull_request_existence_check", trace, err)
				return
			}

			if !exists {
				err = failure.NotFound("pull_request")
				err = failure.Wrap("github.database.pull_request.existence_check", err)
				errlog.Http(logger, w, "pull_request", failure.NotFoundError, err, trace)
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
				err = failure.Wrap("github.database.pull_request.comment.insert", err)
				errlog.Database(logger, w, "pull_request_comment_insert", trace, err)
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
			err = failure.Wrap("github.database.issue.existence_check", err)
			errlog.Database(logger, w, "issue_existence_check", trace, err)
			return
		}

		if !exists {
			err = failure.NotFound("issue")
			err = failure.Wrap("github.database.issue.existence_check", err)
			errlog.Http(logger, w, "issue", failure.NotFoundError, err, trace)
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
			err = failure.Wrap("github.database.issue.comment.insert", err)
			errlog.Database(logger, w, "issue_comment_insert", trace, err)
			return
		}

		logger.Info().
			Str("code", "success").
			Msg("successfully inserted issue comment")
		lib.Pretty(w, http.StatusOK, nil)
	}
}
