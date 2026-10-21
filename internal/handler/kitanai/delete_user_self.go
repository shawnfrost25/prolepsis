package kitanai

import (
	"context"
	"net/http"
	"time"

	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/lib"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgtype"
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
			err = failure.Wrap("kitanai.redis.session.id.delete", err)
			errlog.Redis(logger, w, "session_id_delete", err, u.Trace)
			return true
		}
		return false
	}

	deleteSessionTokens := func(ctx context.Context, id string) bool {
		tokens, err := h.RedisClient.SMembers(ctx, "session:id:"+id).Result()
		if err != nil {
			err = failure.Wrap("kitanai.redis.session.id.fetch", err)
			errlog.Redis(logger, w, "session_id_fetch", err, u.Trace)
			return true
		}

		for _, token := range tokens {
			_, err = h.RedisClient.Del(ctx, "session:token:"+token).Result()
			if err != nil {
				err = failure.Wrap("kitanai.redis.session.token.delete", err)
				errlog.Redis(logger, w, "session_token_delete", err, u.Trace)
				return true
			}
		}
		return false
	}

	setDeletionAlarm := func(ctx context.Context, scheduled_at pgtype.Timestamptz, userID pgtype.UUID) bool {
		err := riverClient.SetupDeletionAlarm(ctx, scheduled_at, userID)
		if err != nil {
			err = failure.Wrap("kitanai.river.deletion.alarm.setup", err)
			errlog.General(logger, w, "deletion_alarm_setup", failure.InternalGeneralError, err, u.Trace)
			return true
		}
		return false
	}

	var req DeleteRequestSelf
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = failure.Wrap("kitanai.json.delete.payload.decode", err)
		errlog.Json(logger, w, "user_delete", u.Trace, err)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	logger.Info().Msg("user is attempting to delete their own account")

	if req.Acceptance != "CONFIRM" {
		err = failure.Forbidden("user_delete", "the CONFIRM keyword was not provided")
		err = failure.Wrap("kitanai.http.delete.confirmation.validate", err)
		errlog.Http(logger, w, "user_delete", failure.ForbiddenError, err, u.Trace)
		return
	}

	if u.Role == "admin" {
		remaining, err := h.Queries.CheckAdminCount(timeout)
		if err != nil {
			err = failure.Wrap("kitanai.database.admin.count_check", err)
			errlog.Database(logger, w, "admin_count_check", u.Trace, err)
			return
		}

		// We don't want to fall in a trap where the company remains without admins (though owners are enough, but it's still not bad if you care so much about the admins)
		if remaining <= 1 {
			err = failure.Forbidden("admin_delete", "deleting the last admin would orphan the company")
			err = failure.Wrap("kitanai.http.admin.count.validate", err)
			errlog.Http(logger, w, "admin_delete", failure.ForbiddenError, err, u.Trace)
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

	infoPD, err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
		UserID:        u.ID,
		RequestedBy:   u.ID,
		Clarification: req.Clarification,
	})
	if err != nil {
		err = failure.Wrap("kitanai.database.deletion.request.insert", err)
		errlog.Database(logger, w, "deletion_request_insert", u.Trace, err)
		return
	}

	failedDA := setDeletionAlarm(timeout, infoPD.ScheduledAt, infoPD.UserID)
	if failedDA {
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully registered self-deletion request")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "succeeded",
	})
}
