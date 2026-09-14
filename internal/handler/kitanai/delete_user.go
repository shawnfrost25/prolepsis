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
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

type DeleteRequest struct {
	Acceptance    string `json:"status"`
	Clarification string `json:"clarification"`
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
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

	var req DeleteRequest
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

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_id_parameter").
			Msg("missing required path parameter")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given parameter for 'id' is empty",
			Details: map[string]string{
				"reason": "missing value for the 'id' parameter",
				"fix":    "Include the value for the missing parameter",
			},
			TraceID: u.Trace,
		})
		return
	}

	var id pgtype.UUID
	err = id.Scan(idStr)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_id_parsing").
			Str("input", idStr).
			Msg("failed to parse string id into uuid type")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Couldn't successfully parse the given id",
			Details: map[string]string{
				"reason": "invalid given id format",
				"fix":    "please, check the token to be right, or send trace",
			},
			TraceID: u.Trace,
		})
		return
	}

	logger = logger.With().Str("target_id", id.String()).Logger()

	infoUS, err := h.Queries.GetUserByID(timeout, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_not_found").
				Msg("target user not found in database")

			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "The given id doesn't match any user in the database",
				Details: map[string]string{
					"reason": "no rows in the database match the given ID",
				},
				TraceID: u.Trace,
			})
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timed out while searching for the user matching the id")

			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while validating the target user",
				Details: map[string]string{
					"reason": "database timeout",
					"fix":    "Please try again later",
				},
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unexpected error while fetching user from database")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An unrecognized error appeared while matching the ID",
			Details: map[string]string{
				"reason": "unrecognized system error",
			},
			TraceID: u.Trace,
		})
		return
	}

	// Welp, the owner can delete everyone and evreything without grace time
	if u.Role == "owner" {
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
			if errors.Is(err, pgconn.ErrConnClosed) {
				logger.Error().Err(err).Str("cause", "timeout").Msg("timed out while owner tried to delete user")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "An error occurred while trying to delete the user from the database",
					TraceID: u.Trace,
				})
				return
			}
			logger.Error().Err(err).Str("cause", "unrecognized").Msg("unknown error while owner tried to delete user")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error while trying to delete the user from the database",
				TraceID: u.Trace,
			})
			return
		}

		logger.Info().Msg("owner successfully deleted all target user sessions directly")
		lib.Pretty(w, http.StatusNoContent, lib.Error{
			Code:    "NO_CONTENT",
			Message: "User successfully deleted",
		})
		return
	}

	// This works for self-deletion
	if u.ID == id {
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
		return
	}

	// User cannot delete nobody (beside themselves)
	if u.Role == "user" {
		logger.Warn().
			Int("status", http.StatusForbidden).
			Str("cause", "no_permissions_for_action").
			Msg("unauthorized attempt by a regular user to delete another user's account")

		lib.Pretty(w, http.StatusForbidden, lib.Error{
			Code:    "FORBIDDEN",
			Message: "You do not have permission to delete another user's account",
			Details: map[string]string{
				"reason": "insufficient permissions",
				"fix":    "you may only initiate deletion for your own account",
			},
			TraceID: u.Trace,
		})
		return
	}

	// An admin cannot delete the owner or another admin
	if u.Role == "admin" {
		if infoUS.Role == "owner" || infoUS.Role == "admin" {
			logger.Warn().
				Int("status", http.StatusForbidden).
				Str("cause", "hierarchy_violation").
				Msg("admin attempted to delete an account with equal or higher privileges")

			lib.Pretty(w, http.StatusForbidden, lib.Error{
				Code:    "FORBIDDEN",
				Message: "Cannot proceed with the action, not enough permissions",
				TraceID: u.Trace,
			})
			return
		}

		if infoUS.Role == "worker" {
			logger.Info().
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
			logger.Error().Err(err).Msg("admin failed to delete user account directly")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while trying to delete the user from the database",
				TraceID: u.Trace,
			})
			return
		}

		logger.Info().Msg("admin successfully deleted target user account directly")
		lib.Pretty(w, http.StatusOK, map[string]string{
			"status": "succeeded",
		})
		return
	}

	if u.Role == "worker" {
		if infoUS.Role != "user" {
			logger.Warn().
				Int("status", http.StatusForbidden).
				Str("cause", "hierarchy_violation").
				Msg("worker attempted to delete a user with equal or higher privileges")

			lib.Pretty(w, http.StatusForbidden, lib.Error{
				Code:    "FORBIDDEN",
				Message: "Cannot proceed with the action, not enough permissions",
				TraceID: u.Trace,
			})
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

		logger.Info().Msg("worker successfully inserted target user into pending deletions")
		lib.Pretty(w, http.StatusOK, map[string]string{
			"status": "succeeded",
		})
		return
	}

	logger.Error().Msg("unrecognized role reached the end of the deletion handler")
	lib.Pretty(w, http.StatusForbidden, lib.Error{
		Code:    "FORBIDDEN",
		Message: "Unrecognized role permissions",
		TraceID: u.Trace,
	})
}
