package kitanai_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/handler/kitanai"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestGetUserByID(t *testing.T) {
	var _ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}

	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(timeout, config)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	auth.Init(queries)

	h := kitanai.New(nil, queries, nil, nil)

	err = h.Queries.TruncateEverythingBeforeTest(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully delete the users and the sessions, due to error: %v", err)
	}
	err = h.Queries.InsertDummiesInsideUsers(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the database due to this error: %v", err)
	}
	err = h.Queries.InsertDummiesInsideSessions(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users session inside the database due to this error: %v", err)
	}

	test := []struct {
		name      string
		method    string
		endpoint  string
		authToken string
		failed    bool
		body      any
	}{
		{
			name:      "Searching for user number 2 (Sheena Totsuki)",
			method:    "GET",
			endpoint:  "/users/00000000-0000-0000-0000-000000000001",
			authToken: "Bearer 00000000000000000000000000000002",
			failed:    false,
			body:      nil,
		},
		{
			name:      "Searching for the last user (Loki Laufeyson)",
			method:    "GET",
			endpoint:  "/users/00000000-0000-0000-0000-000000000024",
			authToken: "Bearer 00000000000000000000000000000024",
			failed:    false,
			body:      nil,
		},
		{
			name:      "Failing by searching an unexistent user",
			method:    "GET",
			endpoint:  "/users/00000000-0000-0000-0000-0000000000999",
			authToken: "Bearer 000000000000000000000000000000999",
			failed:    true,
			body:      nil,
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Get("/users/{id}", h.GetUserByID)

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				bodyBytes, err := sonic.Marshal(tt.body)
				if err != nil {
					t.Errorf("expected status 200, but got error: %v", err)
					return
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, body)
			req.Header.Set("Authorization", tt.authToken)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == http.StatusOK {
					t.Errorf("Expected failure status, but got 200")
				}
				return
			}

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
