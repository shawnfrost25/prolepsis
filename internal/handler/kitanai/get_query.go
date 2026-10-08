package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

type User struct {
	ID          pgtype.UUID        `json:"id"`
	Name        string             `json:"name"`
	DisplayName *string            `json:"display_name,omitempty"`
	Bio         *string            `json:"bio,omitempty"`
	Sex         string             `json:"sex"`
	Location    *string            `json:"location,omitempty"`
	BirthDate   pgtype.Date        `json:"birth_date"`
	Email       string             `json:"email,omitempty"`
	Role        string             `json:"role"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

func (h *Handler) GetUserByQuery(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "get_users_by_query").Logger()
	u, ok := auth.FetchContextInsideHandler(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	displayQuery := q.Get("display_name")
	locationQuery := q.Get("location")
	sexQuery := q.Get("sex")
	birthQuery := q.Get("birth_date")
	roleQuery := q.Get("role")

	logger.Debug().
		Bool("display_name_filter", displayQuery != "").
		Bool("location_filter", locationQuery != "").
		Bool("sex_filter", sexQuery != "").
		Bool("birth_filter", birthQuery != "").
		Bool("role_filter", roleQuery != "").
		Msg("processing get user by query request")

	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	data := psql.Select("id", "name", "display_name", "bio", "sex", "location", "birth_date", "email", "role", "created_at").From("users")
	if displayQuery != "" {
		data = data.Where(squirrel.Eq{"display_name": displayQuery})
	}
	if locationQuery != "" {
		data = data.Where(squirrel.Eq{"location": locationQuery})
	}
	if sexQuery != "" {
		data = data.Where(squirrel.Eq{"sex": sexQuery})
	}
	if birthQuery != "" {
		data = data.Where(squirrel.Eq{"birth_date": birthQuery})
	}
	if roleQuery != "" {
		data = data.Where(squirrel.Eq{"role": roleQuery})
	}

	sqlCode, args, err := data.ToSql()
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "sql_builder_failed").
			Msg("sql generation failed")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal error occurred",
			TraceID: u.Trace,
		})
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var pgErr *pgconn.PgError
	rows, err := h.Pool.Query(timeout, sqlCode, args...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "database_query", w, u.Trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, u.Trace, "database_query_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "database_query", w, u.Trace, err)
		return
	}
	defer rows.Close()

	var result []User
	for rows.Next() {
		var us User
		err := rows.Scan(&us.ID, &us.Name, &us.DisplayName, &us.Bio, &us.Sex, &us.Location, &us.BirthDate, &us.Email, &us.Role, &us.CreatedAt)
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("code", "user_row_scan_failed").
				Msg("user scan failed")

			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An internal error occurred",
				TraceID: u.Trace,
			})
			return
		}

		if string(u.Role) == "user" {
			us.Email = ""
			us.BirthDate = pgtype.Date{}
		}

		result = append(result, us)
	}

	if err := rows.Err(); err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("code", "user_row_iteration_failed").
			Msg("user iteration failed")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal error occurred",
			TraceID: u.Trace,
		})
		return
	}

	if len(result) == 0 {
		errlog.NoRowsError(logger, "user", w, u.Trace)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully found users matching the query")

	lib.Pretty(w, http.StatusOK, result)
}
