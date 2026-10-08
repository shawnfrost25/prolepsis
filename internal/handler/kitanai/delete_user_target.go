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
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type DeleteRequestTarget struct {
	Clarification string `json:"clarification"`
}

func (h *Handler) DeleteUserTarget(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "DeleteUser").Logger()
	riverClient := kitanaijob.New(h.RiverClient, h.Queries)

	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	deleteSessionID := func(ctx context.Context, id string) bool {
		_, err := h.RedisClient.Del(ctx, "session:id:"+id).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "session_id_deletion", w, u.Trace, err)
				return true
			}
			if redis.IsAuthError(err) {
				errlog.RedisAuthenticationError(logger, w, u.Trace, err)
				return true
			}
			errlog.UnexpectedError(logger, "session_id_deletion", w, u.Trace, err)
			return true
		}
		return false
	}

	deleteSessionTokens := func(ctx context.Context, id string) bool {
		tokens, err := h.RedisClient.SMembers(ctx, "session:id:"+id).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "session_id_fetching", w, u.Trace, err)
				return true
			}
			if redis.IsAuthError(err) {
				errlog.RedisAuthenticationError(logger, w, u.Trace, err)
				return true
			}
			errlog.UnexpectedError(logger, "session_id_fetching", w, u.Trace, err)
			return true
		}

		for _, token := range tokens {
			_, err := h.RedisClient.Del(ctx, "session:token:"+token).Result()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errlog.DeadlineExceededError(logger, "session_token_deletion", w, u.Trace, err)
					return true
				}
				if redis.IsAuthError(err) {
					errlog.RedisAuthenticationError(logger, w, u.Trace, err)
					return true
				}
				errlog.UnexpectedError(logger, "session_token_deletion", w, u.Trace, err)
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

	var req DeleteRequestTarget
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errlog.DecodeError(logger, w, u.Trace, err)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		errlog.EmptyValueError(logger, "id_parameter", w, u.Trace)
		return
	}

	var id pgtype.UUID
	err = id.Scan(idStr)
	if err != nil {
		errlog.ParsingError(logger, "id_format", w, u.Trace, err)
		return
	}

	logger = logger.With().Str("target_id", id.String()).Logger()
	var pgErr *pgconn.PgError

	infoUS, err := h.Queries.GetUserByID(timeout, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			errlog.NoRowsError(logger, "user", w, u.Trace)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_lookup", w, u.Trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, u.Trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, u.Trace, "user_lookup_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "user_lookup", w, u.Trace, err)
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
			if errors.Is(err, pgx.ErrNoRows) {
				errlog.NoRowsError(logger, "user", w, u.Trace)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "user_deletion", w, u.Trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, u.Trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, u.Trace, "user_deletion_failed", pgErr)
				return
			}
			errlog.UnexpectedError(logger, "user_deletion", w, u.Trace, err)
			return
		}
		logger.Info().Int("status", http.StatusNoContent).
			Str("code", "user_deleted").
			Msg("owner successfully deleted target")
		lib.Pretty(w, http.StatusNoContent, nil)
		return

		// User cannot delete nobody (beside themselves)
	case "user":
		errlog.ForbiddenError(logger, "user_deletion", w, u.Trace)
		return

	// An admin cannot delete the owner or another admin
	case "admin":
		if infoUS.Role == "owner" || infoUS.Role == "admin" {
			errlog.ForbiddenError(logger, "user_deletion", w, u.Trace)
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
				if errors.Is(err, context.DeadlineExceeded) {
					errlog.DeadlineExceededError(logger, "user_deletion_request", w, u.Trace, err)
					return
				}
				if errors.Is(err, pgconn.ErrConnClosed) {
					errlog.ErrConnClosed(logger, w, u.Trace, err)
					return
				}
				if errors.As(err, &pgErr) {
					errlog.PgConnError(logger, w, u.Trace, "user_deletion_request_failed", pgErr)
					return
				}
				errlog.UnexpectedError(logger, "user_deletion_request", w, u.Trace, err)
				return
			}

			failedDA := setDeletionAlarm(timeout, infoPD.ScheduledAt, infoPD.UserID)
			if failedDA {
				return
			}

			logger.Info().
				Int("status", http.StatusOK).
				Str("code", "inserted_pending_deletion").
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
			if errors.Is(err, pgx.ErrNoRows) {
				errlog.NoRowsError(logger, "user", w, u.Trace)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "user_deletion", w, u.Trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, u.Trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, u.Trace, "user_deletion_failed", pgErr)
				return
			}
			errlog.UnexpectedError(logger, "user_deletion", w, u.Trace, err)
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
			errlog.ForbiddenError(logger, "user_deletion", w, u.Trace)
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
			if errors.Is(err, context.DeadlineExceeded) {
				errlog.DeadlineExceededError(logger, "user_deletion_request", w, u.Trace, err)
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				errlog.ErrConnClosed(logger, w, u.Trace, err)
				return
			}
			if errors.As(err, &pgErr) {
				errlog.PgConnError(logger, w, u.Trace, "user_deletion_request_failed", pgErr)
				return
			}
			errlog.UnexpectedError(logger, "user_deletion_request", w, u.Trace, err)
			return
		}

		failedDA := setDeletionAlarm(timeout, infoPD.ScheduledAt, infoPD.UserID)
		if failedDA {
			return
		}

		logger.Info().
			Int("status", http.StatusOK).
			Str("code", "inserted_deletion_request").
			Str("worker_id", u.ID.String()).
			Msg("worker successfully inserted deletion request")
		lib.Pretty(w, http.StatusOK, map[string]string{
			"status": "succeeded",
		})
		return
	}

	errlog.UnauthorizedError(logger, "nonexistent_role", w, u.Trace, nil)
}
