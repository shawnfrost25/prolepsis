package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

type UpdateRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Location    *string `json:"location"`
}

func (h *Handler) UpdateUserInfo(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "UpdateUserInfo").Logger()
	u, ok := auth.FetchContext(w, r)
	if !ok {
		return
	}
	var req UpdateRequest
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_decode_request").
			Msg("couldn't decode request")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given request is invalid. Failed to decode the request",
			Details: map[string]string{
				"reason": "the given request is malformed/not supported for the structure",
			},
			TraceID: u.Trace,
		})
		return
	}

	if req.Bio == nil && req.DisplayName == nil && req.Location == nil {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "empty_update_payload").
			Str("trace_id", u.Trace).
			Msg("no update fields provided in request")

		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "At least one field (bio, display_name, or location) must be provided",
			Details: map[string]string{
				"reason": "payload contains no updatable fields",
			},
			TraceID: u.Trace,
		})
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	err = h.Queries.UpdateUserNotForced(timeout, db.UpdateUserNotForcedParams{
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
		Location:    req.Location,
		ID:          u.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_id_not_found").
				Str("trace_id", u.Trace).
				Msg("user profile not found for update")

			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "The specified user account was not found",
				Details: map[string]string{
					"reason": "no record matches the given user ID",
				},
				TraceID: u.Trace,
			})
			return
		}

		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "uppdate_user_error").
			Str("trace_id", u.Trace).
			Msg("failed to update user profile in database")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An error occurred while updating your profile",
			Details: map[string]string{
				"reason": "database write operation failed",
				"fix":    "Please try again later",
			},
			TraceID: u.Trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully update user info")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "successful",
	})
}
