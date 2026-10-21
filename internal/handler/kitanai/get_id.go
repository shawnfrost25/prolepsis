package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/go-chi/chi/v5"
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
		err := failure.EmptyValue("id_parameter")
		err = failure.Wrap("kitanai.http.parameter.id.validate", err)
		errlog.Http(logger, w, "id_parameter", failure.EmptyValueError, err, u.Trace)
		return
	}

	var id pgtype.UUID
	err := id.Scan(idRaw)
	if err != nil {
		err = failure.Wrap("kitanai.http.parameter.id.convert", err)
		errlog.Http(logger, w, "id_parameter", failure.MalformedError, err, u.Trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	info, err := h.Queries.GetUserByID(timeout, id)
	if err != nil {
		err = failure.Wrap("kitanai.database.user.fetch_via_id", err)
		errlog.Database(logger, w, "user_lookup", u.Trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("cause", "success").
		Msg("successfully matched the given ID")

	lib.Pretty(w, http.StatusOK, info)
}
