package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
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
		err = failure.Wrap("kitanai.json.update.payload.decode", err)
		errlog.Json(logger, w, "user_update", u.Trace, err)
		return
	}

	if req.Bio == nil && req.DisplayName == nil && req.Location == nil {
		err = failure.EmptyValue("user_update_fields")
		err = failure.Wrap("kitanai.http.update.payload.existence_check", err)
		errlog.Http(logger, w, "user_update_fields", failure.EmptyValueError, err, u.Trace)
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
		err = failure.Wrap("kitanai.database.user.update", err)
		errlog.Database(logger, w, "user_update", u.Trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully update user info")

	lib.Pretty(w, http.StatusOK, map[string]string{
		"status": "successful",
	})
}
