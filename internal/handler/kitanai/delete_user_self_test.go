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

type deleteRequestSelf struct {
	Acceptance    string `json:"status"`
	Clarification string `json:"clarification"`
}

func dus(t *testing.T) *Server {
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
	auth.Init(queries, rdb)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Delete("/users/delete/self", h.DeleteUserSelf)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestDeleteUserSelf(t *testing.T) {
	s := dus(t)

	sheenaID := s.seed(t, "Sheena Totsuki", "user")
	seiranID := s.seed(t, "Lizzy Seiran", "admin")

	sheenaToken := s.login(t, sheenaID)
	seiranToken := s.login(t, seiranID)

	test := []struct {
		Name           string
		ExpectedStatus int
		Token          string
		Body           deleteRequestSelf
	}{
		{
			Name:           "Successfully accepting Sheena's self-deletion request",
			ExpectedStatus: http.StatusOK,
			Token:          sheenaToken,
			Body: deleteRequestSelf{
				Acceptance:    "CONFIRM",
				Clarification: "Didn't really enjoy the app... I wanna spend time with Mimi",
			},
		},
		{
			Name:           "Failing due to missing confirmation",
			ExpectedStatus: http.StatusForbidden,
			Token:          sheenaToken,
			Body: deleteRequestSelf{
				Acceptance:    "",
				Clarification: "Mmm, I don't like this app",
			},
		},
		{
			Name:           "Failing due to orphaned company risk",
			ExpectedStatus: http.StatusForbidden,
			Token:          seiranToken,
			Body: deleteRequestSelf{
				Acceptance:    "CONFIRM",
				Clarification: "Ari...",
			},
		},
		{
			Name:           "Failing due to missing token",
			ExpectedStatus: http.StatusUnauthorized,
			Token:          "",
			Body: deleteRequestSelf{
				Acceptance:    "CONFIRM",
				Clarification: "",
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodDelete, "/users/delete/self", tt.Token, tt.Body)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
