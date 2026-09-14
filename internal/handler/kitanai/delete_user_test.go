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

func TestDeleteUSer(t *testing.T) {
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

	h := kitanai.New(nil, queries, nil, nil)
	auth.Init(queries)

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
		idParam   string
		authToken string
		failed    bool
		body      map[string]any
	}{
		{
			name:      "Trying to delete Asya Shubina (as Asya Shubina, a self-deletion)",
			method:    "DELETE",
			endpoint:  "/users/delete/00000000-0000-0000-0000-000000000009",
			authToken: "Bearer 00000000000000000000000000000009",
			failed:    false,
			body: map[string]any{
				"status":        "CONFIRM",
				"clarification": "Uhm, the app sucks, like...a lot, slop",
			},
		},
		{
			name:      "Trying to delete Hinako (owner) as Airi (user) - Failed",
			method:    "DELETE",
			endpoint:  "/users/delete/00000000-0000-0000-0000-000000000006",
			authToken: "Bearer 00000000000000000000000000000005",
			failed:    true,
			body: map[string]any{
				"clarification": "Got ostracized due to her, delete her",
			},
		},
		{
			name:      "Trying to delete Ethan Winters (worker) as Chris Redfield (admin)",
			method:    "DELETE",
			endpoint:  "/users/delete/00000000-0000-0000-0000-000000000015",
			authToken: "Bearer 00000000000000000000000000000016",
			failed:    false,
			body: map[string]any{
				// UHM, SPOILERS (He's not dead, k?)
				"clarification": "Ethan, confirmed death after the encounter with Miranda, inactive account",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Delete("/users/delete/{id}", h.DeleteUser)

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
			w := httptest.NewRecorder()
			req.Header.Set("Authorization", tt.authToken)

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 200 {
					t.Error("Expected failure but got status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected status 201; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
