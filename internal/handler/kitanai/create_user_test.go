package kitanai

import (
	"context"
	"net/http"
	"os"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"
)

type UserPayload struct {
	Name      string      `json:"name"`
	Sex       string      `json:"sex"`
	BirthDate pgtype.Date `json:"birth_date"`
	Email     string      `json:"email"`
	Password  string      `json:"password"`
}

func cu(t *testing.T) *Server {
	rdb, pool, queries := testenv.New(t)
	_ = godotenv.Load("../../../.env")
	appPassword := os.Getenv("APPPASSWORD")
	email := os.Getenv("EMAIL")

	mailer := &Mailer{
		AppPassword: appPassword,
		Email:       email,
	}

	h := New(nil, queries, mailer, nil, rdb, nil)
	r := chi.NewRouter()
	auth.Init(queries, rdb)

	r.Group(func(r chi.Router) {
		r.Use(auth.Logger_Middleware)
		r.Post("/users/create", h.CreateUser)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestCreateUser(t *testing.T) {
	s := cu(t)

	s.seed(t, "Ari Maud")
	_ = s.red.Set(context.Background(), "pending:registration:email:"+"arimaud@gmail.com", 1, 24*time.Hour)

	test := []struct {
		Name           string
		Body           UserPayload
		ExpectedStatus int
	}{
		{
			Name: "Successfully creating a user",
			Body: UserPayload{
				Name:      "Sheena Totsuki",
				Sex:       "female",
				BirthDate: pgtype.Date{Time: time.Date(2012, 11, 25, 1, 1, 1, 1, time.Local), Valid: true},
				Email:     "sheenatotsuki@gmail.com",
				Password:  "sheena_standing_png",
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name: "Failing due to missing field",
			Body: UserPayload{
				Name:      "Mimi Kagari",
				Sex:       "female",
				BirthDate: pgtype.Date{},
				Email:     "mimikagari@gmail.com",
				Password:  "mimi_is_mimi",
			},
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name: "Failing due to rate limit",
			Body: UserPayload{
				Name:      "Ari Maud",
				Sex:       "female",
				BirthDate: pgtype.Date{Time: time.Date(2012, 05, 01, 1, 1, 1, 1, time.Local), Valid: true},
				Email:     "arimaud@gmail.com",
				Password:  "absence",
			},
			ExpectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodPost, "/users/create", "", tt.Body)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
