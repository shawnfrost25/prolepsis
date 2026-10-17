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
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h *WebhookHandler) WebhookPullRequest(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "pull_request").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

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

	var synErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	var req PullRequestWebhook
	err = sonic.Unmarshal(body, &req)
	if err != nil {
		if errors.As(err, &synErr) {
			errlog.JsonSyntaxError(logger, "pull_request_payload", w, trace, synErr)
			return
		}
		if errors.As(err, &typeErr) {
			errlog.JsonUnmarshalTypeError(logger, w, trace, typeErr)
		}
		errlog.ParsingError(logger, "pull_request_payload", w, trace, err)
		return
	}

	closed := req.PullRequest.ClosedAt != nil

	if strings.ToLower(req.PullRequest.State) != "open" && strings.ToLower(req.PullRequest.State) != "closed" {
		errlog.InvalidError(logger, "pull_request_state", w, trace)
		return
	}

	var authLogin *string
	if req.PullRequest.Author != nil {
		authLogin = &req.PullRequest.Author.Login
	}

	var closedAt pgtype.Timestamptz
	if req.PullRequest.ClosedAt != nil {
		closedAt = pgtype.Timestamptz{Time: *req.PullRequest.ClosedAt, Valid: true}
	}

	var mergedAt pgtype.Timestamptz
	if req.PullRequest.MergedAt != nil {
		mergedAt = pgtype.Timestamptz{Time: *req.PullRequest.MergedAt, Valid: true}
	}

	var labels []string
	if len(req.PullRequest.Labels) != 0 {
		for _, label := range req.PullRequest.Labels {
			if label.Name == "" {
				continue
			}
			labels = append(labels, label.Name)
		}
	}

	err = h.Queries.InsertPullRequest(timeout, db.InsertPullRequestParams{
		GithubRepoID: req.Repository.ID,
		GithubUserID: req.Repository.Owner.ID,
		Action:       &req.Action,
		AuthorLogin:  authLogin,
		Title:        req.PullRequest.Title,
		Body:         req.PullRequest.Body,
		State:        req.PullRequest.State,
		Labels:       labels,
		Additions:    int32(req.PullRequest.Additions),
		ChangedFiles: int32(req.PullRequest.ChangedFiles),
		Deletions:    int32(req.PullRequest.Deletions),
		Number:       int32(req.PullRequest.Number),
		IsClosed:     closed,
		IsDraft:      req.PullRequest.IsDraft,
		IsLocked:     req.PullRequest.IsLocked,
		IsMerged:     req.PullRequest.IsMerged,
		HtmlUrl:      req.PullRequest.HtmlUrl,
		CreatedAt:    pgtype.Timestamptz{Time: req.PullRequest.CreatedAt},
		UpdatedAt:    pgtype.Timestamptz{Time: req.PullRequest.UpdatedAt},
		ClosedAt:     closedAt,
		MergedAt:     mergedAt,
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
		Msg("successfully inserted commits and pr")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
