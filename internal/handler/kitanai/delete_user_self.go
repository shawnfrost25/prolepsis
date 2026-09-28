package kitanai

import (
	"context"
	"errors"
	"net/http"
	"time"

	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/lib"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

type DeleteRequestSelf struct {
	Acceptance    string `json:"status"`
	Clarification string `json:"clarification"`
}

func (h *Handler) DeleteUserSelf(w http.ResponseWriter, r *http.Request) {
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
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("timedout while trying to delete user's session (id) inside Redis")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Couldn't continue due to timeout while deleting user's session",
					TraceID: u.Trace,
				})
				return true
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("unrecognized error while trying to delete user's session (id)")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error while deleting user's session",
				TraceID: u.Trace,
			})
			return true
		}

		return false
	}

	deleteSessionTokens := func(ctx context.Context, id string) bool {
		tokens, err := h.RedisClient.SMembers(ctx, "session:id:"+id).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("timedout while trying to fetch all the sessions connected to the given id")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timeout while trying to match the given id to across multiple sessions",
					TraceID: u.Trace,
				})
				return true
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("unrecognized error while trying to query through sessions with the provided id")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unspecified error while querying through the available sessions",
				TraceID: u.Trace,
			})
			return true
		}
		if len(tokens) == 0 {
			logger.Warn().
				Int("status", http.StatusNotFound).
				Str("cause", "session_not_found").
				Str("provided_id", id).
				Msg("the provided id doesn't match any session inside Redis")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "The provided id is invalid",
				TraceID: u.Trace,
			})
			return true
		}

		for _, token := range tokens {
			_, err = h.RedisClient.Del(ctx, "session:token:"+token).Result()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					logger.Error().
						Err(err).
						Int("status", http.StatusRequestTimeout).
						Str("cause", "timeout").
						Msg("timedout while trying to delete user's session (token) inside Redis")
					lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
						Code:    "REQUEST_TIMEOUT",
						Message: "Couldn't continue due to timeout while deleting user's session",
						TraceID: u.Trace,
					})
					return true
				}
				logger.Error().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("cause", "unrecognized").
					Msg("unrecognized error while trying to delete user's session (token)")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Unrecognized error while deleting user's session",
					TraceID: u.Trace,
				})
				return true
			}
		}

		return false
	}

	setDeletionAlarm := func(ctx context.Context, scheduled_at pgtype.Timestamptz, userID pgtype.UUID) bool {
		err := riverClient.SetupDeletionAlarm(ctx, scheduled_at, userID)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("river alarm ended up timing out, retry")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Couldn't start the alarm due to timeout",
					TraceID: u.Trace,
				})
				return true
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("unrecognized while trying to start the alarm")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Couldn't start alarm due to unrecognized error",
				TraceID: u.Trace,
			})
			return true
		}

		return false
	}

	var req DeleteRequestSelf
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_decode_request").
			Msg("couldn't decode request payload")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given request is invalid. Failed to decode the request.",
			Details: map[string]string{
				"reason": "invalid request payload",
				"fix":    "follow the recommendations and given format to successfully continue",
			},
			TraceID: u.Trace,
		})
		return
	}

	if req.Clarification == "" {
		req.Clarification = "Unknown"
	}

	timeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	logger.Info().Msg("user is attempting to delete their own account")

	if req.Acceptance != "CONFIRM" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_confirmation").
			Msg("deletion request rejected due to missing 'CONFIRM' status")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "No confirmation was given to continue the deletion",
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
			logger.Error().Err(err).Msg("failed to evaluate remaining admins during self-deletion")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unexpected error while evaluating remaining admins",
				TraceID: u.Trace,
			})
			return
		}

		// We don't want to fall in a trap where the company remains without admins (though owners are enough, but it's still not bad if you care so much about the admins)
		if remaining <= 1 {
			logger.Warn().
				Int("status", http.StatusForbidden).
				Str("cause", "orphaned_company_risk").
				Msg("prevented lockout... last admin attempted self-deletion")

			lib.Pretty(w, http.StatusForbidden, lib.Error{
				Code:    "FORBIDDEN",
				Message: "Uhm, no, just no. The company needs to grow! Deleting this account will cause an administrative lockout, which means a mountain of trouble later. Let's just keep it, okay?",
				Details: map[string]string{
					"reason": "last admin deletion",
					"fix":    "keep those pretty hands by yourself and let at least an account...pwease",
				},
				TraceID: u.Trace,
			})
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
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timedout while trying to inssert the user inside the pending_deletion")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to insert the reuqest inside the pending deletions",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("failed to insert self-deletion request into pending deletions")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An error occurred while trying to register the deletion request",
			TraceID: u.Trace,
		})
		return
	}

	failedDA := setDeletionAlarm(timeout, infoPD.ScheduledAt, infoPD.UserID)
	if failedDA {
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("cause", "successfully_inserted_pending_deletion").
		Msg("successfully registered self-deletion request")
	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "succeeded",
	})
}
