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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestCreateUser(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		t.Fatal("missing 'RESEND_API_KEY' inside .env")
	}
	email := os.Getenv("FROM_EMAIL")
	if email == "" {
		t.Fatal("missing 'FROM_EMAIL' inside .env")
	}
	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

	value := kitanai.Mailer{
		ApiKey: apiKey,
	}

	queries := db.New(pool)
	h := kitanai.New(pool, queries, &value)
	auth.Init(queries)

	var yachiyoDate pgtype.Date
	// It should've been 08-32 - not 31!!!
	if err := yachiyoDate.Scan("1970-08-31"); err != nil {
		t.Fatalf("failed to parse date: %v", err)
	}

	var kokoaDate pgtype.Date
	if err := kokoaDate.Scan("2007-12-06"); err != nil {
		t.Fatalf("failed to parse date: %v", err)
	}

	test := []struct {
		name     string
		method   string
		endpoint string
		failed   bool
		body     map[string]any
	}{
		{
			name:     "Creating the 8000 years old lady (Yachiyo Runami)",
			method:   "POST",
			endpoint: "/users/create",
			failed:   false,
			body: map[string]any{
				"name":       "Yachiyo Runami",
				"sex":        "female",
				"birth_date": yachiyoDate,
				"email":      email,
				// anime reference
				"password": "i-am-not-the-kaguya-you-know-anymore",
			},
		},
		{
			name:     "Using an invalid field to blow everything up",
			method:   "POST",
			endpoint: "/users/create",
			failed:   true,
			body: map[string]any{
				"name":       "Kokoa Yoshizaki",
				"sex":        "sweetheart",
				"birth_date": kokoaDate,
				"email":      nil,
				// manga reference
				"password": "i-am-no-goddess",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Post("/users/create", h.CreateUser)

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				bodyBytes, err := sonic.Marshal(tt.body)
				if err != nil {
					t.Errorf("Expected to turn body into bytes, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, body)
			w := httptest.NewRecorder()
			req.Header.Set("Authorization", "Bearer 711a89dd67a2fe9e0e8a0972dd3847e29c0bda32bc2cd8b671b4a277dda9c896")

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 200 {
					t.Errorf("Expected failure, but got status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected status 200; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
