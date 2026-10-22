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

func (h WebhookHandler) WebhookRelease(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "releases").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("github.http.trace.context.fetch", err)
		errlog.Internal(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req ReleasesWebhook
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		err = failure.Wrap("github.json.release.payload.unmarshal", err)
		errlog.Json(logger, w, "release", trace, err)
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
		err = failure.Wrap("github.database.release.insert", err)
		errlog.Database(logger, w, "release_insert", trace, err)
		return
	}

	logger.Info().
		Str("cause", "success").
		Msg("successfully inserted releases")

	lib.Pretty(w, http.StatusOK, nil)
}
