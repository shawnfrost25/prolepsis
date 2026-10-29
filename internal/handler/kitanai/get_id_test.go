package kitanai

import (
	"encoding/json"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func newStart(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	h := New(nil, queries, nil, nil, nil, nil)
	r := chi.NewRouter()
	auth.Init(queries, rdb)
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Get("/users/{id}", h.GetUserByID)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestGetUserById(t *testing.T) {
	s := newStart(t)

	mimiID := s.seed(t, "Mimi Kagari")
	sheenaID := s.seed(t, "Sheena Totsuki")

	generalToken := s.login(t, mimiID)

	test := []struct {
		Name           string
		Endpoint       string
		Token          string
		ExpectedStatus int
		ExpectedID     pgtype.UUID
	}{
		{
			Name:           "successfully fetching Sheena Totsuki",
			Endpoint:       "/users/" + sheenaID.String(),
			Token:          generalToken,
			ExpectedStatus: 200,
			ExpectedID:     sheenaID,
		},
		{
			Name:           "successfully fetching Mimi Kagari",
			Endpoint:       "/users/" + mimiID.String(),
			Token:          generalToken,
			ExpectedStatus: 200,
			ExpectedID:     mimiID,
		},
		{
			Name:           "failind due to non-existent id",
			Endpoint:       "/users/7ca58bef-1d42-49ba-9376-c1de5502f838",
			Token:          generalToken,
			ExpectedStatus: 404,
		},
		{
			Name:           "failed due to malformed id",
			Endpoint:       "/users/invalid_id",
			Token:          generalToken,
			ExpectedStatus: 400,
		},
		{
			Name:           "failing due to missing auth token",
			Endpoint:       "/users/" + mimiID.String(),
			Token:          "",
			ExpectedStatus: 401,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodGet, tt.Endpoint, tt.Token, nil)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}

			if tt.ExpectedStatus == http.StatusOK {
				var resp struct {
					ID string `json:"id"`
				}

				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
				}

				if resp.ID != tt.ExpectedID.String() {
					t.Fatalf("Givend id: %s \nExpected id: %s \nResponse: %s", resp.ID, tt.ExpectedID.String(), w.Body.String())
				}
			}
		})
	}
}
