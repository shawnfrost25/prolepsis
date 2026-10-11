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

func (h WebhookHandler) WebhookReleases(w http.ResponseWriter, r *http.Request, body []byte, deliveryID string) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "releases").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.InternalEmptyValueError(logger, "trace_context", w, trace)
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req GitHubReleases
	err := sonic.Unmarshal(body, &req)
	if err != nil {
		errlog.ParsingError(logger, "releases_format", w, trace, err)
		return
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

	hookID, err := strconv.ParseInt(hookIDStr, 10, 64)
	if err != nil {
		errlog.ParsingError(logger, "hook_id", w, trace, err)
		return
	}

	var publishedAt pgtype.Timestamptz
	if req.Release.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: *req.Release.PublishedAt, Valid: true}
	}

	var pgErr *pgconn.PgError

	err = h.Queries.InsertGitHubReleases(timeout, db.InsertGitHubReleasesParams{
		GithubRepoID:    req.Repository.ID,
		GithubUserID:    req.Repository.OwnerID.ID,
		ReleasesID:      req.Release.ID,
		HookID:          hookID,
		Action:          req.Action,
		TagName:         req.Release.TagName,
		Name:            req.Release.Name,
		Body:            req.Release.Body,
		TargetCommitish: req.Release.TargetCommitish,
		Draft:           req.Release.Draft,
		Prerelease:      req.Release.Prerelease,
		PublishedAt:     publishedAt,
		SenderID:        req.Sender.ID,
		SenderName:      req.Sender.Login,
		SenderType:      req.Sender.Type,
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
