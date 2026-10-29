package kitanai

import (
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"

	"github.com/go-chi/chi/v5"
)

func gubq(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	h := New(pool, nil, nil, nil, nil, nil)
	auth.Init(queries, rdb)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Get("/users", h.GetUserByQuery)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestGetUserByQuery(t *testing.T) {
	s := gubq(t)

	mimiId := s.seed(t, "Mimi Kagari", "")
	s.seed(t, "Sheena Totsuki", "")
	s.seed(t, "Ari Maud", "")

	generalToken := s.login(t, mimiId)

	test := []struct {
		Name           string
		Endpoint       string
		Token          string
		ExpectedStatus int
	}{
		{
			Name:           "Successfully fetch Ari Maud",
			Endpoint:       "/users?display_name=Ari%20Maud",
			Token:          generalToken,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Successufully fetch all the users",
			Endpoint:       "/users?sex=prefer_not_to_specify",
			Token:          generalToken,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Failing due to missing query",
			Endpoint:       `/users`,
			Token:          generalToken,
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "Failing due to unexistent user",
			Endpoint:       "/users?display_name=Lizzy%20Seiran",
			Token:          generalToken,
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Failing due to missing token",
			Endpoint:       "/users?email=mimikagari@gmail.com",
			Token:          "",
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodGet, tt.Endpoint, tt.Token, nil)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
