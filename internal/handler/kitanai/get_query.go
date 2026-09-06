package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/lib"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
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
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "GetUserByQuery").Logger()
	u, ok := auth.FetchContext(w, r)
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
		Str("display_name_filter", displayQuery).
		Str("location_filter", locationQuery).
		Str("sex_filter", sexQuery).
		Str("birth_filter", birthQuery).
		Str("role_filter", roleQuery).
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
			Str("cause", "sql_builder_failed").
			Msg("failed to generate SQL query statement")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to generate SQL query statement",
			TraceID: u.Trace,
		})
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()

	rows, err := h.pool.Query(timeout, sqlCode, args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_query_not_found").
				Msg("user not found in database")

			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "The given query doesn't match any user in the database",
				Details: map[string]string{
					"reason": "no rows in the database match the given query",
				},
				TraceID: u.Trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "database_query_failed").
			Str("sql", sqlCode).
			Msg("failed to fetch rows based on provided info")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to fetch rows based on provided info",
			TraceID: u.Trace,
		})
		return
	}

	var result []User
	for rows.Next() {
		var us User
		err := rows.Scan(&us.Name, &us.DisplayName, &us.Bio, &us.Sex, &us.Location, &us.BirthDate, &us.Email, &us.Role, &us.CreatedAt)
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "row_scan_failed").
				Msg("failed to parse database records cleanly")

			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Failed to parse database records cleanly",
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
			Str("cause", "row_iteration_failed").
			Msg("error occurred during row iteration")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error reading database rows",
			TraceID: u.Trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully found users matching the query")

	lib.Pretty(w, http.StatusOK, result)
}
