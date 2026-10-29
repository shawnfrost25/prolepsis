package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func vu(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	h := New(nil, queries, nil, nil, rdb, nil)
	auth.Init(queries, rdb)
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(auth.Logger_Middleware)
		r.Get("/users/create/verify", h.VerifyRegistration)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestVerifyRegistration(t *testing.T) {
	s := vu(t)

	_ = s.red.HSetEXWithArgs(context.Background(), "pending:registration:token:"+"5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5", &redis.HSetEXOptions{
		Condition:      "FNX",
		ExpirationType: redis.HSetEXExpirationEX,
		ExpirationVal:  300,
	},
		"name", "Sheena Totsuki",
		"sex", "female",
		"birth_date", time.DateOnly,
		"email", "sheenatotsuki@gmail.com",
		"password_hash", "I_know_it_is_long_but_it_is_forced_since_it_gotta_be_60_characters_long",
	)

	test := []struct {
		Name           string
		Endpoint       string
		ExpectedStatus int
	}{
		{
			Name:           "Successfully verify Sheena Totsuki",
			Endpoint:       "/users/create/verify?token=12345",
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:           "Failing due to non-existent token",
			Endpoint:       "/users/create/verify?token=23456",
			ExpectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodGet, tt.Endpoint, "", nil)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
