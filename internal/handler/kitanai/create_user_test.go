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

func cu(t *testing.T) *Server {
	t.Helper()
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

	ptrStr := func(str string) *string {
		return &str
	}

	ptrDate := func() *pgtype.Date {
		return &pgtype.Date{Time: time.Date(2012, 11, 25, 1, 1, 1, 1, time.Local), Valid: true}
	}

	s.seed(t, "Ari Maud", "")
	_, err := s.red.Set(context.Background(), "pending:registration:email:"+"arimaud@gmail.com", 1, 24*time.Hour).Result()
	if err != nil {
		t.Fatalf("Failed to add rate limit: %v", err)
	}

	test := []struct {
		Name           string
		Body           RegisterUser
		ExpectedStatus int
	}{
		{
			Name: "Successfully creating a user",
			Body: RegisterUser{
				Name:      ptrStr("Sheena Totsuki"),
				Sex:       ptrStr("female"),
				BirthDate: ptrDate(),
				Email:     ptrStr("sheenatotsuki@gmail.com"),
				Password:  ptrStr("sheena_standing_png"),
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name: "Failing due to missing field",
			Body: RegisterUser{
				Name:      ptrStr("Mimi Kagari"),
				Sex:       ptrStr("female"),
				BirthDate: &pgtype.Date{},
				Email:     ptrStr("mimikagari@gmail.com"),
				Password:  ptrStr("mimi_is_mimi"),
			},
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name: "Failing due to rate limit",
			Body: RegisterUser{
				Name:      ptrStr("Ari Maud"),
				Sex:       ptrStr("female"),
				BirthDate: ptrDate(),
				Email:     ptrStr("arimaud@gmail.com"),
				Password:  ptrStr("absence"),
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
