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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func (h *Handler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "get_user_by_id").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		// The function already handles the response and logging
		return
	}

	idRaw := chi.URLParam(r, "id")

	if idRaw == "" {
		logger.Warn().
			Str("code", "missing_id_parameter").
			Int("status", http.StatusBadRequest).
			Msg("request rejected")

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
	err := id.Scan(idRaw)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("code", "invalid_id_format").
			Str("provided_id", idRaw).
			Msg("request rejected")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Couldn't successfully parse the given id",
			Details: map[string]string{
				"reason": "invalid given id",
				"fix":    "please, check the token to be right, or send trace",
			},
			TraceID: u.Trace,
		})
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	var pgErr *pgconn.PgError

	info, err := h.Queries.GetUserByID(timeout, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Int("status", http.StatusNotFound).
				Str("code", "user_not_found").
				Msg("user not found")
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
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusGatewayTimeout).
				Str("code", "user_lookup_timeout").
				Msg("user lookup timed out")
			lib.Pretty(w, http.StatusGatewayTimeout, lib.Error{
				Code:    "GATEWAY_TIMEOUT",
				Message: "The user lookup timed out",
				Details: map[string]string{
					"reason": "The user lookup was stopped due to timeout",
					"fix":    "Please try again later",
				},
				TraceID: u.Trace,
			})
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("code", "database_connection_closed").
				Msg("database connection closed")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed user lookup",
				TraceID: u.Trace,
			})
			return
		}
		if errors.As(err, &pgErr) {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("code", "database_query_failed").
				Str("constraint_name", pgErr.ConstraintName).
				Str("database_status_code", pgErr.Code).
				Str("database_detail", pgErr.Detail).
				Str("database_hint", pgErr.Hint).
				Msg("database failed")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An internal error occurred",
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "user_lookup_failed").
			Str("user_id", id.String()).
			Msg("failed to retrieve user")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal error occurred",
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
