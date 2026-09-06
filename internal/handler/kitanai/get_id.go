package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/lib"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

func (h *Handler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "GetUserByID").Logger()
	u, ok := auth.FetchContext(w, r)
	if !ok {
		// The function already handles the response and logging
		return
	}

	id := chi.URLParam(r, "id")

	if id == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_id_parameter").
			Msg("invalid request")

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

	timeout, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()

	info, err := h.queries.GetUserByID(timeout, u.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_id_not_found").
				Str("input", id).
				Msg("user not found in database")

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
				Str("input", id).
				Msg("timedout while searching for the user matching the id")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while validating the token",
				Details: map[string]string{
					"reason": "database error",
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
			Str("input", id).
			Msg("unexptected error while matching the ID through the database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An unrecognized error appeared while matching the ID",
			Details: map[string]string{
				"reason": "unrecognized",
			},
			TraceID: u.Trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("cause", "success").
		Msg("successfully matched the given ID")

	lib.Pretty(w, http.StatusOK, info)
}
