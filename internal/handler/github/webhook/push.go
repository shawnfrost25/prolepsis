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

func (h *WebhookHandler) WebhookPush(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "push").Logger()
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

	var req GitHubPush
	err = sonic.Unmarshal(body, &req)
	if err != nil {
		err = failure.Wrap("github.json.push.payload.unmarshal", err)
		errlog.Json(logger, w, "push", trace, err)
		return
	}

	const zeroSHA = "0000000000000000000000000000000000000000"

	isZero := func(sha *string) *string {
		if sha == nil || *sha == "" || *sha == zeroSHA {
			return nil
		}
		return sha
	}

	pushID, err := h.Queries.InsertPush(timeout, db.InsertPushParams{
		GithubRepoID: req.RepoInfo.ID,
		GithubUserID: req.RepoInfo.Owner.ID,
		Ref:          req.Ref,
		BeforeSha:    isZero(&req.BeforeSHA),
		AfterSha:     isZero(&req.AfterSHA),
		IsForced:     req.Forced,
		IsCreated:    req.Created,
	})
	if err != nil {
		err = failure.Wrap("github.database.push.insert", err)
		errlog.Database(logger, w, "push_insert", trace, err)
		return
	}

	for _, commit := range req.CommitInfo {
		err := h.Queries.InsertPushCommit(timeout, db.InsertPushCommitParams{
			PushID:      pushID,
			CommitSha:   commit.CommitSha,
			Message:     commit.Message,
			Added:       commit.Added,
			Removed:     commit.Removed,
			Modified:    commit.Modified,
			CommittedAt: pgtype.Timestamptz{Time: commit.CommittedAt, Valid: true},
		})
		if err != nil {
			err = failure.Wrap("github.database.push.commit.insert", err)
			errlog.Database(logger, w, "push_commit_insert", trace, err)
			return
		}

		logger.Debug().
			Str("commit_sha", commit.CommitSha).
			Str("push_id", pushID.String()).
			Msg("successfully inserted commit")
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully managed to insert the webhook push info and the commits too")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
