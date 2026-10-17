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

func (h WebhookHandler) WebhookReleases(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "releases").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req ReleasesWebhook
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "releases_format", w, trace, err)
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
	if req.Release.Author != nil {
		authorLogin = &req.Release.Author.Login
	}

	var createdAt pgtype.Timestamptz
	if req.Release.CreatedAt != nil {
		createdAt = pgtype.Timestamptz{Time: *req.Release.CreatedAt, Valid: true}
	}

	var updatedAt pgtype.Timestamptz
	if req.Release.UpdatedAt != nil {
		updatedAt = pgtype.Timestamptz{Time: *req.Release.UpdatedAt, Valid: true}
	}

	var publishedAt pgtype.Timestamptz
	if req.Release.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: *req.Release.PublishedAt, Valid: true}
	}

	err = h.Queries.InsertRelease(timeout, db.InsertReleaseParams{
		ID:            req.Release.Id,
		GithubRepoID:  req.Repository.ID,
		GithubUserID:  req.Repository.Owner.ID,
		Action:        &req.Action,
		AuthorLogin:   authorLogin,
		Name:          req.Release.Name,
		TagName:       req.Release.TagName,
		Description:   req.Release.Description,
		IsDraft:       req.Release.IsDraft,
		IsPrereleased: req.Release.IsPrereleased,
		IsImmutable:   req.Release.IsImmutable,
		HtmlUrl:       req.Release.HtmlUrl,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		PublishedAt:   publishedAt,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "releases_insert", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "releases_insert_failed", pgErr, err)
			return
		}
		errlog.UnexpectedError(logger, "releases_insert", w, trace, err)
		return
	}

	logger.Info().
		Str("cause", "success").
		Msg("successfully inserted releases")

	lib.Pretty(w, http.StatusOK, nil)
}
