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

func (h *WebhookHandler) WebhookIssues(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "issues").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("github.http.trace.context.fetch", err)
		errlog.Internal(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req IssuesWebhook
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		err = failure.Wrap("github.json.issue.payload.unmarshal", err)
		errlog.Json(logger, w, "issue", trace, err)
		return
	}

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

	if req.Issue.Title == "" {
		err = failure.EmptyValue("issue_title")
		err = failure.Wrap("github.http.issue.title.existence_check", err)
		errlog.Http(logger, w, "issue_title", failure.EmptyValueError, err, trace)
		return
	}

	positiveReactions := req.Issue.Reactions.Hooray + req.Issue.Reactions.Positive + req.Issue.Reactions.Heart + req.Issue.Reactions.Laugh + req.Issue.Reactions.Rocket
	negativeReactions := req.Issue.Reactions.Confused + req.Issue.Reactions.Negative

	if req.Issue.State != "open" && req.Issue.State != "closed" {
		err = failure.Invalid("issue_state", "unsupported state "+req.Issue.State)
		err = failure.Wrap("github.http.issue.state.validate", err)
		errlog.Http(logger, w, "issue_state", failure.InvalidError, err, trace)
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
		err = failure.Wrap("github.database.issue.insert", err)
		errlog.Database(logger, w, "issue_insert", trace, err)
		return
	}

	logger.Info().
		Str("code", "success").
		Msg("issue inserted")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"reason": "success",
	})
}
