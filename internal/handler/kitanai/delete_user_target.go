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
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

type DeleteRequestTarget struct {
	Clarification string `json:"clarification"`
}

func (h *Handler) DeleteUserTarget(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "delete_user").Logger()
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
			_, err := h.RedisClient.Del(ctx, "session:token:"+token).Result()
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

	var req DeleteRequestTarget
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = failure.Wrap("kitanai.json.delete.payload.decode", err)
		errlog.Json(logger, w, "user_delete", u.Trace, err)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		err = failure.EmptyValue("id_parameter")
		err = failure.Wrap("kitanai.http.parameter.id.validate", err)
		errlog.Http(logger, w, "id_parameter", failure.EmptyValueError, err, u.Trace)
		return
	}

	var id pgtype.UUID
	err = id.Scan(idStr)
	if err != nil {
		err = failure.Wrap("kitanai.http.parameter.id.convert", err)
		errlog.Http(logger, w, "id_parameter", failure.MalformedError, err, u.Trace)
		return
	}

	logger = logger.With().Str("target_id", id.String()).Logger()

	infoUS, err := h.Queries.GetUserByID(timeout, id)
	if err != nil {
		err = failure.Wrap("kitanai.database.user.fetch_via_id", err)
		errlog.Database(logger, w, "user_lookup", u.Trace, err)
		return
	}

	// Welp, the owner can delete everyone and evreything without grace time
	switch u.Role {
	case "owner":
		failedTokens := deleteSessionTokens(timeout, idStr)
		if failedTokens {
			// We handle the errors inside the closure
			return
		}
		failedID := deleteSessionID(timeout, idStr)
		if failedID {
			return
		}
		err := h.Queries.DeleteUser(timeout, id)
		if err != nil {
			err = failure.Wrap("kitanai.database.user.delete", err)
			errlog.Database(logger, w, "user_delete", u.Trace, err)
			return
		}
		logger.Info().Int("status", http.StatusNoContent).
			Str("code", "user_deleted").
			Msg("owner successfully deleted target")
		lib.Pretty(w, http.StatusNoContent, nil)
		return

		// User cannot delete nobody (beside themselves)
	case "user":
		err = failure.Forbidden("user_delete", "users cannot delete other accounts")
		err = failure.Wrap("kitanai.http.role.permission.validate", err)
		errlog.Http(logger, w, "user_delete", failure.ForbiddenError, err, u.Trace)
		return

	// An admin cannot delete the owner or another admin
	case "admin":
		if infoUS.Role == "owner" || infoUS.Role == "admin" {
			err = failure.Forbidden("user_delete", "admins cannot delete owners or other admins")
			err = failure.Wrap("kitanai.http.role.permission.validate", err)
			errlog.Http(logger, w, "user_delete", failure.ForbiddenError, err, u.Trace)
			return
		}

		if infoUS.Role == "worker" {
			logger.Debug().
				Str("target_name", infoUS.Name).
				Msg("admin requested the deletion of a worker, inserting into pending deletions")

			failedTokens := deleteSessionTokens(timeout, idStr)
			if failedTokens {
				return
			}

			failedID := deleteSessionID(timeout, idStr)
			if failedID {
				return
			}

			infoPD, err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
				UserID:        id,
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
				Str("code", "pending_deletion_inset").
				Msg("successfully registered request")
			lib.Pretty(w, http.StatusOK, map[string]string{
				"status": "succeeded",
			})
			return
		}

		// If it's an user
		failedTokens := deleteSessionTokens(timeout, idStr)
		if failedTokens {
			return
		}

		failedID := deleteSessionID(timeout, idStr)
		if failedID {
			return
		}

		err = h.Queries.DeleteUser(timeout, id)
		if err != nil {
			err = failure.Wrap("kitanai.database.user.delete", err)
			errlog.Database(logger, w, "user_delete", u.Trace, err)
			return
		}

		logger.Info().
			Int("status", http.StatusOK).
			Str("code", "user_deleted").
			Str("admin_id", u.ID.String()).
			Msg("admin successfully deleted target")
		lib.Pretty(w, http.StatusOK, nil)
		return

	case "worker":
		if infoUS.Role != "user" {
			err = failure.Forbidden("user_delete", "workers can only delete users")
			err = failure.Wrap("kitanai.http.role.permission.validate", err)
			errlog.Http(logger, w, "user_delete", failure.ForbiddenError, err, u.Trace)
			return
		}

		failedTokens := deleteSessionTokens(timeout, idStr)
		if failedTokens {
			return
		}

		failedID := deleteSessionID(timeout, idStr)
		if failedID {
			return
		}

		infoPD, err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
			UserID:        id,
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
			Str("worker_id", u.ID.String()).
			Msg("worker successfully inserted deletion request")
		lib.Pretty(w, http.StatusOK, map[string]string{
			"status": "succeeded",
		})
		return
	}

	err = failure.Unauthorized("role", "the role is not recognized")
	err = failure.Wrap("kitanai.http.role.permission.validate", err)
	errlog.Http(logger, w, "role", failure.UnauthorizedError, err, u.Trace)
}
