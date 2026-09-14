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

func TestVerifyRegistration(t *testing.T) {
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
	err = h.Queries.InsertDummiesInsidePendingRegistrations(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the pending registartions due to this error: %v", err)
	}

	err = h.Queries.InsertDummiesInsideSentEmails(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the sent_emails table due to this error: %v", err)
	}

	test := []struct {
		name     string
		method   string
		endpoint string
		failed   bool
		body     map[string]any
	}{
		{
			name:     "Trying to verify if Hatsune Miku is valid",
			method:   "POST",
			endpoint: "/users/create/verify",
			failed:   false,
			body: map[string]any{
				"token": "pending0000000000000000000000002",
			},
		},
		{
			name:     "Failing due to invalid token",
			method:   "POST",
			endpoint: "/users/create/verify",
			failed:   true,
			body: map[string]any{
				"token": "pending0000000000000000000000999",
			},
		},
		{
			name:     "Failing due to missing token",
			method:   "POST",
			endpoint: "/users/create/verify",
			failed:   true,
			body: map[string]any{
				"token": "",
			},
		},
		{
			name:     "Failing due to expired token (Kasane Teto)",
			method:   "POST",
			endpoint: "/users/create/verify",
			failed:   true,
			body: map[string]any{
				"token": "pending0000000000000000000000003",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Logger_Middleware)
	r.Post("/users/create/verify", h.VerifyRegistration)

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				s, err := sonic.Marshal(tt.body)
				if err != nil {
					t.Errorf("Expected everything to decode, got %v", err)
				}
				body = bytes.NewReader(s)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, body)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 201 {
					t.Error("Expected failure, but user got created anyways, proof: status 201")
				}
				return
			}

			if w.Code != 201 {
				t.Errorf("Expected status 201; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
