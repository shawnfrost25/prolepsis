package kitanai

import (
	"net/http"
	"prolepsis/internal/auth"
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/testenv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type deleteRequestTarget struct {
	Clarification string `json:"clarification"`
}

func dut(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	workers := kitanaijob.SetupDeletionWorkers(queries)

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create river client: %v", err)
	}

	h := New(nil, queries, nil, nil, rdb, riverClient)
	r := chi.NewRouter()
	auth.Init(queries, rdb)
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Delete("/users/delete/target/{id}", h.DeleteUserTarget)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestDeleteUserTarget(t *testing.T) {
	s := dut(t)

	mimiID := s.seed(t, "Mimi Kagari", "admin")
	sheenaID := s.seed(t, "Sheena Totsuki", "worker")
	ariID := s.seed(t, "Ari Maud", "user")

	mimiToken := s.login(t, mimiID)
	sheenaToken := s.login(t, sheenaID)
	ariToken := s.login(t, ariID)

	test := []struct {
		Name           string
		Endpoint       string
		Token          string
		ExpectedStatus int
		Body           deleteRequestTarget
	}{
		{
			Name:           "Fail: Worker -> Admin",
			Endpoint:       "/users/delete/target/" + mimiID.String(),
			ExpectedStatus: http.StatusForbidden,
			Token:          sheenaToken,
			Body: deleteRequestTarget{
				Clarification: "I mean... she've been spamming buttons...",
			},
		},
		{
			Name:           "Fail: User -> Worker",
			Endpoint:       "/users/delete/target/" + sheenaID.String(),
			Token:          ariToken,
			ExpectedStatus: http.StatusForbidden,
			Body:           deleteRequestTarget{},
		},
		{
			Name:           "Fail: Admin -> Admin",
			Endpoint:       "/users/delete/target/" + mimiID.String(),
			Token:          mimiToken,
			ExpectedStatus: http.StatusForbidden,
			Body: deleteRequestTarget{
				Clarification: "Mimi doesn't understand what this means... scribble scribble",
			},
		},
		{
			Name:           "Success: Worker -> User",
			Endpoint:       "/users/delete/target/" + ariID.String(),
			Token:          sheenaToken,
			ExpectedStatus: http.StatusOK,
			Body:           deleteRequestTarget{},
		},
		{
			Name:           "Success: Admin Worker",
			Endpoint:       "/users/delete/target/" + sheenaID.String(),
			Token:          mimiToken,
			ExpectedStatus: http.StatusOK,
			Body: deleteRequestTarget{
				Clarification: "Ohh, pretty red button... Mimi wanna click it...",
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodDelete, tt.Endpoint, tt.Token, tt.Body)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
