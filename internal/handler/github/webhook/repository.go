package webhook

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h WebhookHandler) WebhookRepository(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "repository").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("github.http.trace.context.fetch", err)
		errlog.Internal(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

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

	var req RepositoryWebhook
	err = sonic.Unmarshal(body, &req)
	if err != nil {
		err = failure.Wrap("github.json.repository.payload.unmarshal", err)
		errlog.Json(logger, w, "repository", trace, err)
		return
	}

	if req.IsFork {
		logger.Debug().
			Int("status", http.StatusOK).
			Str("code", "repository_fork").
			Msg("repository is a fork")
		lib.Pretty(w, http.StatusOK, nil)
		return
	}

	repoIDStr := strconv.FormatInt(req.ID, 10)
	hookIDStr, err := h.RedisClient.Get(timeout, "oauth:github:webhook:hook_id:"+repoIDStr).Result()
	if err != nil {
		err = failure.Wrap("github.redis.webhook.hook_id.fetch", err)
		errlog.Redis(logger, w, "webhook_id_fetch", err, trace)
		return
	}

	if hookIDStr == "" {
		err = failure.EmptyValue("webhook_id")
		err = failure.Wrap("github.redis.webhook.hook_id.existence_check", err)
		errlog.Http(logger, w, "webhook_id", failure.EmptyValueError, err, trace)
		return
	}

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		err = failure.Wrap("github.http.webhook.id.convert", err)
		errlog.Http(logger, w, "webhook_id", failure.MalformedError, err, trace)
		return
	}

	var licenseKey *string
	if req.LicenseInfo != nil {
		err := h.Queries.InsertGitHubLicense(timeout, db.InsertGitHubLicenseParams{
			SpdxID:     req.LicenseInfo.SpdxId,
			NodeID:     req.LicenseInfo.NodeId,
			LicenseKey: req.LicenseInfo.Key,
			Name:       req.LicenseInfo.Name,
			Body:       req.LicenseInfo.Body,
			HtmlUrl:    req.LicenseInfo.HtmlUrl,
		})
		if err != nil {
			err = failure.Wrap("github.database.license.insert", err)
			errlog.Database(logger, w, "license_insert", trace, err)
			return
		}

		licenseKey = &req.LicenseInfo.Key
		logger.Debug().
			Str("license_key", req.LicenseInfo.Key).
			Str("license_name", req.LicenseInfo.Name).
			Msg("inserted new license")
	}

	var createdAt pgtype.Timestamptz
	if req.CreatedAt != nil {
		createdAt = pgtype.Timestamptz{Time: *req.CreatedAt, Valid: true}
	}

	var pushedAt pgtype.Timestamptz
	if req.PushedAt != nil {
		pushedAt = pgtype.Timestamptz{Time: *req.PushedAt, Valid: true}
	}

	var updatedAt pgtype.Timestamptz
	if req.UpdatedAt != nil {
		updatedAt = pgtype.Timestamptz{Time: *req.UpdatedAt, Valid: true}
	}

	err = h.Queries.InsertGitHubRepo(timeout, db.InsertGitHubRepoParams{
		GithubID:     req.Owner.ID,
		RepoID:       req.ID,
		HookID:       hookID,
		Action:       &req.Action,
		Name:         req.Name,
		Description:  req.Description,
		MainLanguage: req.Language,
		LicenseKey:   licenseKey,
		Visibility:   *req.Visibility,
		Stars:        int32(req.Stars),
		Watches:      int32(req.Watches),
		ForksCount:   int32(req.ForksCount),
		IsArchived:   req.IsArchived,
		HtmlUrl:      req.HtmlUrl,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		PushedAt:     pushedAt,
	})
	if err != nil {
		err = failure.Wrap("github.database.repository.insert", err)
		errlog.Database(logger, w, "repository_insert", trace, err)
		return
	}

	logger.Info().
		Str("code", "success").
		Msg("successfully inserted repository")
	lib.Pretty(w, http.StatusOK, nil)
}
