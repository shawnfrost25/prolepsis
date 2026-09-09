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

	h := kitanai.New(pool, queries, nil)

	test := []struct {
		name     string
		method   string
		endpoint string
		idParam  string
		failed   bool
		body     any
	}{
		{
			name:     "Searching for user number 2",
			method:   "GET",
			endpoint: "/users/7f1829b7-5b69-4dc9-8a49-f3cd393e1653",
			idParam:  "7f1829b7-5b69-4dc9-8a49-f3cd393e1653",
			failed:   false,
			body:     nil,
		},
		{
			name:     "Searching for the last user",
			method:   "GET",
			endpoint: "/users/d4d1f675-6453-410e-9693-86885d842ed1",
			idParam:  "d4d1f675-6453-410e-9693-86885d842ed1",
			failed:   false,
			body:     nil,
		},
		{
			name:     "Failing by searching an unexistent user",
			method:   "GET",
			endpoint: "/users/999asd",
			idParam:  "99asda9",
			failed:   true,
			body:     nil,
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
			req.Header.Set("Authorization", "Bearer 711a89dd67a2fe9e0e8a0972dd3847e29c0bda32bc2cd8b671b4a277dda9c896")
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
