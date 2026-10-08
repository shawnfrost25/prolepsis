package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
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
		errlog.EmptyValueError(logger, "id_parameter", w, u.Trace)
		return
	}

	var id pgtype.UUID
	err := id.Scan(idRaw)
	if err != nil {
		errlog.ParsingError(logger, "uuid_format", w, u.Trace, err)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	var pgErr *pgconn.PgError

	info, err := h.Queries.GetUserByID(timeout, id)
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

	logger.Info().
		Int("status", http.StatusOK).
		Str("cause", "success").
		Msg("successfully matched the given ID")

	lib.Pretty(w, http.StatusOK, info)
}
