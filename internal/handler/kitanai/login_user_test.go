package kitanai

import (
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"

	"github.com/go-chi/chi/v5"
)

func lu(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	h := New(nil, queries, nil, nil, rdb, nil)
	auth.Init(queries, rdb)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Logger_Middleware)
		r.Post("/users/login", h.LoginUser)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestLoginUser(t *testing.T) {
	s := lu(t)

	_ = s.seed(t, "Mimi Kagari", "")

	test := []struct {
		Name           string
		ExpectedStatus int
		Body           LoginRequest
	}{
		{
			Name:           "Successfully logging",
			ExpectedStatus: http.StatusOK,
			Body: LoginRequest{
				Email:    "mimikagari@gmail.com",
				Password: "Sheena",
			},
		},
		{
			Name:           "Failing due to non-exisiting account",
			ExpectedStatus: http.StatusNotFound,
			Body: LoginRequest{
				Email:    "sheenatotsuki@gmail.com",
				Password: "Sheena",
			},
		},
		{
			Name:           "Failing due to invalid password",
			ExpectedStatus: http.StatusUnauthorized,
			Body: LoginRequest{
				Email:    "mimikagari@gmail.com",
				Password: "Mimi",
			},
		},
		{
			Name:           "Failing due to missing body",
			ExpectedStatus: http.StatusBadRequest,
			Body:           LoginRequest{},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodPost, "/users/login", "", tt.Body)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
