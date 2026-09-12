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
	email := os.Getenv("FROM_EMAIL")
	if email == "" {
		t.Fatal("Missing 'FROM_EMAIL' inside .env")
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

	h := kitanai.New(pool, queries, nil, nil)

	test := []struct {
		name     string
		method   string
		endpoint string
		failed   bool
		body     map[string]any
	}{
		{
			name:     "Loging in as myself",
			method:   "POST",
			endpoint: "/users/login",
			failed:   false,
			body: map[string]any{
				"email":    email,
				"password": "mimi-kagari",
			},
		},
		{
			name:     "Trying to with an incorrect password",
			method:   "POST",
			endpoint: "/users/login",
			failed:   true,
			body: map[string]any{
				"email":    email,
				"password": "sheena-totsuki",
			},
		},
	}

	r := chi.NewRouter()
	auth.Init(queries)
	r.Use(auth.Auth_Middleware)
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
			req.Header.Set("Authorization", "Bearer 711a89dd67a2fe9e0e8a0972dd3847e29c0bda32bc2cd8b671b4a277dda9c896")

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
