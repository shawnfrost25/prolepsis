package webhook

import (
	"context"
	"encoding/json"
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

func (h *WebhookHandler) WebhookIssues(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issues").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var synErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	var req IssuesWebhook
	err := sonic.Unmarshal(body, &req)
	if errors.As(err, &synErr) {
		errlog.JsonSyntaxError(logger, "issue_payload", w, trace, synErr)
		return
	}
	if errors.As(err, &typeErr) {
		errlog.JsonUnmarshalTypeError(logger, w, trace, typeErr)
		return
	}
	if err != nil {
		errlog.ParsingError(logger, "issues_payload", w, trace, err)
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

	if req.Issue.Title == "" {
		errlog.EmptyValueError(logger, "issues_title", w, trace)
		return
	}

	positiveReactions := req.Issue.Reactions.Hooray + req.Issue.Reactions.Positive + req.Issue.Reactions.Heart + req.Issue.Reactions.Laugh + req.Issue.Reactions.Rocket
	negativeReactions := req.Issue.Reactions.Confused + req.Issue.Reactions.Negative

	if req.Issue.State != "open" && req.Issue.State != "closed" {
		errlog.InvalidError(logger, "issues_state", w, trace)
		return
	}

	var authorLogin *string
	if req.Issue.Author != nil {
		authorLogin = &req.Issue.Author.Login
	}

	var closedAt pgtype.Timestamptz
	if req.Issue.ClosedAt != nil {
		closedAt = pgtype.Timestamptz{Time: *req.Issue.ClosedAt, Valid: true}
	}

	var issueType *string
	if req.Issue.Type != nil {
		issueType = &req.Issue.Type.Name
	}

	var labels []string
	if len(req.Issue.Labels) != 0 {
		for _, label := range req.Issue.Labels {
			if label.Name == "" {
				continue
			}
			labels = append(labels, label.Name)
		}
	}

	closed := req.Issue.ClosedAt != nil

	err = h.Queries.InsertIssue(timeout, db.InsertIssueParams{
		ID:                req.Issue.Id,
		GithubRepoID:      req.Repository.ID,
		GithubUserID:      req.Repository.Owner.ID,
		Action:            &req.Action,
		AuthorLogin:       authorLogin,
		Title:             req.Issue.Title,
		Body:              *req.Issue.Body,
		State:             req.Issue.State,
		StateReason:       req.Issue.StateReason,
		IssueType:         issueType,
		Labels:            labels,
		Number:            int32(req.Issue.Number),
		PositiveReactions: int32(positiveReactions),
		NegativeReactions: int32(negativeReactions),
		TotalReactions:    int32(req.Issue.Reactions.TotalCount),
		IsClosed:          closed,
		IsLocked:          req.Issue.IsLocked,
		HtmlUrl:           req.Issue.HtmlUrl,
		CreatedAt:         pgtype.Timestamptz{Time: req.Issue.CreatedAt},
		UpdatedAt:         pgtype.Timestamptz{Time: req.Issue.UpdatedAt},
		ClosedAt:          closedAt,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "issues_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "issues_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "issues_insert", w, trace, err)
		return
	}

	logger.Info().
		Str("code", "success").
		Msg("issue inserted")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"reason": "success",
	})
}
