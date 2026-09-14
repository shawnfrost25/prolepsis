package kitanai_test

import (
	"bytes"
	"context"
	"io"
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

func TestLoginUser(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	timeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
		name     string
		method   string
		endpoint string
		failed   bool
		body     map[string]any
	}{
		{
			name:     "Loging in as Hinako Hanamura",
			method:   "POST",
			endpoint: "/users/login",
			failed:   false,
			body: map[string]any{
				"email":    "hinako@test.com",
				"password": "Test1234!",
			},
		},
		{
			name:     "Trying to with an incorrect password",
			method:   "POST",
			endpoint: "/users/login",
			failed:   true,
			body: map[string]any{
				"email":    "hinako@test.com",
				"password": "idk_which",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Logger_Middleware)
	r.Post("/users/login", h.LoginUser)

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				bytesBody, err := sonic.Marshal(tt.body)
				if err != nil {
					t.Errorf("Expected everything to decode, got %v", err)
				}
				body = bytes.NewReader(bytesBody)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, body)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 200 {
					t.Errorf("Expected error, but got status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected status 200; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
