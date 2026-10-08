package kitanai

import (
	"context"
	"errors"
	"net/http"
	"time"

	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/lib"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type DeleteRequestSelf struct {
	Acceptance    string `json:"status"`
	Clarification string `json:"clarification"`
}

func (h *Handler) DeleteUserSelf(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "delete_self").Logger()
	riverClient := kitanaijob.New(h.RiverClient, h.Queries)

	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	deleteSessionID := func(ctx context.Context, id string) bool {
		_, err := h.RedisClient.Del(ctx, "session:id:"+id).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "session_id_delete", w, u.Trace, err)
				return true
			}
			if redis.IsAuthError(err) {
				errlog.RedisAuthenticationError(logger, w, u.Trace, err)
				return true
			}
			errlog.UnexpectedError(logger, "session_id_delete", w, u.Trace, err)
			return true
		}
		return false
	}

	deleteSessionTokens := func(ctx context.Context, id string) bool {
		tokens, err := h.RedisClient.SMembers(ctx, "session:id:"+id).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "session_id_fetch", w, u.Trace, err)
				return true
			}
			if redis.IsAuthError(err) {
				errlog.RedisAuthenticationError(logger, w, u.Trace, err)
				return true
			}
			errlog.UnexpectedError(logger, "session_id_fetch", w, u.Trace, err)
			return true
		}

		for _, token := range tokens {
			_, err = h.RedisClient.Del(ctx, "session:token:"+token).Result()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errlog.DeadlineExceededError(logger, "session_token_delete", w, u.Trace, err)
					return true
				}
				if redis.IsAuthError(err) {
					errlog.RedisAuthenticationError(logger, w, u.Trace, err)
					return true
				}
				errlog.UnexpectedError(logger, "session_token_delete", w, u.Trace, err)
				return true
			}
		}
		return false
	}

	setDeletionAlarm := func(ctx context.Context, scheduled_at pgtype.Timestamptz, userID pgtype.UUID) bool {
		err := riverClient.SetupDeletionAlarm(ctx, scheduled_at, userID)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "river_setup", w, u.Trace, err)
				return true
			}
			errlog.UnexpectedError(logger, "river_setup", w, u.Trace, err)
			return true
		}
		return false
	}

	var req DeleteRequestSelf
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errlog.DecodeError(logger, w, u.Trace, err)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	logger.Info().Msg("user is attempting to delete their own account")

	if req.Acceptance != "CONFIRM" {
		logger.Warn().
			Int("status", http.StatusForbidden).
			Str("code", "missing_confirm").
			Msg("request rejected")
		lib.Pretty(w, http.StatusForbidden, lib.Error{
			Code:    "FORBIDDEN",
			Message: "Invalid request",
			Details: map[string]string{
				"reason": "missing confirmation",
				"fix":    "to delete the account, you must write 'CONFIRM' in the box and accept",
			},
			TraceID: u.Trace,
		})
		return
	}

	if u.Role == "admin" {
		remaining, err := h.Queries.CheckAdminCount(timeout)
		if err != nil {
			errlog.UnexpectedError(logger, "admin_count_check", w, u.Trace, err)
			return
		}

		// We don't want to fall in a trap where the company remains without admins (though owners are enough, but it's still not bad if you care so much about the admins)
		if remaining <= 1 {
			errlog.ForbiddenError(logger, "orphaned_company_risk", w, u.Trace)
			return
		}
	}

	failedTokens := deleteSessionTokens(timeout, u.ID.String())
	if failedTokens {
		return
	}
	failedID := deleteSessionID(timeout, u.ID.String())
	if failedID {
		return
	}

	var pgErr *pgconn.PgError

	infoPD, err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
		UserID:        u.ID,
		RequestedBy:   u.ID,
		Clarification: req.Clarification,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_delete_request", w, u.Trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, u.Trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, u.Trace, "user_delete_request_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "user_delete_request", w, u.Trace, err)
		return
	}

	failedDA := setDeletionAlarm(timeout, infoPD.ScheduledAt, infoPD.UserID)
	if failedDA {
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("code", "inserted_pending_delete").
		Msg("successfully registered self-deletion request")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "succeeded",
	})
}
