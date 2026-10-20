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

func (h *WebhookHandler) WebhookPush(w http.ResponseWriter, r *http.Request, body []byte, deliveryID pgtype.UUID) {
	logger := zerolog.Ctx(r.Context()).With().Str("event", "push").Logger()
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

	var req GitHubPush
	err = sonic.Unmarshal(body, &req)
	if err != nil {
		if errors.As(err, &synErr) {
			errlog.JsonSyntaxError(logger, "push_payload", w, trace, synErr)
			return
		}
		if errors.As(err, &typeErr) {
			errlog.JsonUnmarshalTypeError(logger, w, trace, typeErr)
		}
		errlog.ParsingError(logger, "push_payload", w, trace, err)
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
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "push_insert", w, trace, err)
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "push_insert_failed", pgErr, err)
		}
		errlog.UnexpectedError(logger, "push_insert", w, trace, err)
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
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "push_commit_insert", w, trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, trace, "push_commit_insert_failed", pgErr, err)
				return
			}
			errlog.UnexpectedError(logger, "push_commit_insert", w, trace, err)
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
