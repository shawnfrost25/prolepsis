package kitanai

import (
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"

	"github.com/go-chi/chi/v5"
)

func uu(t *testing.T) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)

	h := New(nil, queries, nil, nil, nil, nil)
	auth.Init(queries, rdb)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Patch("/users/update", h.UpdateUserInfo)
	})

	return ServerNew(r, rdb, queries, pool)
}

func TestUpdateUserInfo(t *testing.T) {
	s := uu(t)

	ptr := func(str string) *string {
		return &str
	}

	mimiID := s.seed(t, "Mimi Kagari", "")
	mimiToken := s.login(t, mimiID)

	test := []struct {
		Name           string
		Token          string
		ExpectedStatus int
		Body           UpdateRequest
	}{
		{
			Name:           "Successfully update all info",
			Token:          mimiToken,
			ExpectedStatus: http.StatusOK,
			Body: UpdateRequest{
				DisplayName: ptr("Mimi_Loves_Sheena"),
				Bio:         ptr("Mimi is Mimi. Mimi likes Sheena and flowery fields!!"),
				Location:    ptr("JP, Idk"),
			},
		},
		{
			Name:           "Failing due to missing fields",
			Token:          mimiToken,
			ExpectedStatus: http.StatusBadRequest,
			Body:           UpdateRequest{},
		},
		{
			Name:           "Failing due to missing token",
			Token:          "",
			ExpectedStatus: http.StatusUnauthorized,
			Body:           UpdateRequest{},
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodPatch, "/users/update", tt.Token, tt.Body)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
