package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/Masterminds/squirrel"
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

	if displayQuery == "" && locationQuery == "" && sexQuery == "" && birthQuery == "" && roleQuery == "" {
		err := failure.EmptyValue("query_parameters")
		err = failure.Wrap("kitanai.http.prameter.query.validate", err)
		errlog.Http(logger, w, "query_parameters", failure.EmptyValueError, err, u.Trace)
		return
	}

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
		err = failure.Wrap("kitanai.squirrel.sql.generate", err)
		errlog.Http(logger, w, "sql_code", failure.GenerationError, err, u.Trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rows, err := h.Pool.Query(timeout, sqlCode, args...)
	if err != nil {
		err = failure.Wrap("kitanai.database.user.fetch_via_query", err)
		errlog.Database(logger, w, "user_query", u.Trace, err)
		return
	}
	defer rows.Close()

	var result []User
	for rows.Next() {
		var us User
		err := rows.Scan(&us.ID, &us.Name, &us.DisplayName, &us.Bio, &us.Sex, &us.Location, &us.BirthDate, &us.Email, &us.Role, &us.CreatedAt)
		if err != nil {
			err = failure.Wrap("kitanai.database.user.scan_rows", err)
			errlog.General(logger, w, "user_row_scan", failure.InternalGeneralError, err, u.Trace)
			return
		}

		if string(u.Role) == "user" {
			us.Email = ""
			us.BirthDate = pgtype.Date{}
		}

		result = append(result, us)
	}

	if err := rows.Err(); err != nil {
		err = failure.Wrap("kitanai.database.user.iterate_rows", err)
		errlog.Database(logger, w, "user_row_iterate", u.Trace, err)
		return
	}

	if len(result) == 0 {
		err = failure.NotFound("user")
		err = failure.Wrap("kitanai.database.user.existence_check", err)
		errlog.Http(logger, w, "user", failure.NotFoundError, err, u.Trace)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully found users matching the query")
	lib.Pretty(w, http.StatusOK, result)
}
