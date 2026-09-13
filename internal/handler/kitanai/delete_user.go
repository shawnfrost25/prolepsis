package kitanai

import (
	"context"
	"errors"
	"net/http"
	"time"

	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
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

	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}

	logger = logger.With().Str("actor_id", u.ID.String()).Str("actor_role", string(u.Role)).Logger()

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

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
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

	info, err := h.Queries.GetUserByID(timeout, id)
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

		logger.Info().Msg("owner successfully deleted the target user directly")
		w.WriteHeader(http.StatusOK)
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

		// InsertDeletionRequest = Insert it in a table and delete after 24 hours (not yet, it will get updated to Redis + asynq later)
		err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
			UserID:        id,
			RequestedBy:   u.ID,
			Clarification: req.Clarification,
		})

		if err != nil {
			logger.Error().Err(err).Msg("failed to insert self-deletion request into pending deletions")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while trying to register the deletion request",
				TraceID: u.Trace,
			})
			return
		}

		logger.Info().Msg("successfully registered self-deletion request")
		w.WriteHeader(http.StatusOK)
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
		if info.Role == "owner" || info.Role == "admin" {
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

		if info.Role == "worker" {
			logger.Info().
				Str("target_name", info.Name).
				Msg("admin requested the deletion of a worker, inserting into pending deletions")

			err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
				UserID:        id,
				RequestedBy:   u.ID,
				Clarification: req.Clarification,
			})

			if err != nil {
				logger.Error().Err(err).Msg("failed to insert worker into pending deletions")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "An error occurred while inserting the worker into pending deletions",
					TraceID: u.Trace,
				})
				return
			}

			logger.Info().Msg("successfully inserted the worker into pending deletions")
			w.WriteHeader(http.StatusOK)
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
		w.WriteHeader(http.StatusOK)
		return
	}

	if u.Role == "worker" {
		if info.Role != "user" {
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

		err := h.Queries.InsertDeletionRequest(timeout, db.InsertDeletionRequestParams{
			UserID:        id,
			RequestedBy:   u.ID,
			Clarification: req.Clarification,
		})

		if err != nil {
			logger.Error().Err(err).Msg("worker failed to insert user into pending deletions")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while inserting the user into pending deletions",
				TraceID: u.Trace,
			})
			return
		}

		logger.Info().Msg("worker successfully inserted target user into pending deletions")
		w.WriteHeader(http.StatusOK)
		return
	}

	logger.Error().Msg("unrecognized role reached the end of the deletion handler")
	lib.Pretty(w, http.StatusForbidden, lib.Error{
		Code:    "FORBIDDEN",
		Message: "Unrecognized role permissions",
		TraceID: u.Trace,
	})
}
