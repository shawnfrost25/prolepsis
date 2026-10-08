package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

type UpdateRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Location    *string `json:"location"`
}

func (h *Handler) UpdateUserInfo(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "update_user_info").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}
	var req UpdateRequest
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errlog.DecodeError(logger, w, u.Trace, err)
		return
	}

	if req.Bio == nil && req.DisplayName == nil && req.Location == nil {
		errlog.EmptyValueError(logger, "update_payload", w, u.Trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var pgErr *pgconn.PgError
	err = h.Queries.UpdateUserNotForced(timeout, db.UpdateUserNotForcedParams{
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
		Location:    req.Location,
		ID:          u.ID,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_update", w, u.Trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, u.Trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, u.Trace, "user_update_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "user_update", w, u.Trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully update user info")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "successful",
	})
}
