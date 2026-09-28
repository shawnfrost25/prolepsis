package oauthGithub_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	oauthGithub "prolepsis/internal/handler/github"
	"prolepsis/internal/lib"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

func TestGitHubCallback(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("Missing 'REDIS_ADDR' inside .env")
	}
	redisPSWD := os.Getenv("REDIS_PASSWORD")
	if redisPSWD == "" {
		t.Fatal("Missing 'REDIS_PASSWORD' inside .env")
	}
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		t.Fatal("Missing 'GITHUB_TOKEN' inside .env")
	}

	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conf, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("Expected to get the database configuration, but got: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(timeout, conf)
	if err != nil {
		t.Fatalf("Expected to get the connection pool, but got: %v", err)
	}
	queries := db.New(pool)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPSWD,
		DB:       0,
	})

	httpClient := http.Client{Timeout: 10 * time.Second}

	h := oauthGithub.GH_New(redisClient, oauth2.Config{}, httpClient, queries)
	auth.Init(queries, redisClient)

	err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/create_session.redis")
	if err != nil {
		t.Fatalf("Ecxpected to create sessions, but ended up failing due to error: %v", err)
	}
	defer func() {
		err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/clean_session.redis")
		if err != nil {
			t.Fatalf("Couldn't successfully clean mock session token, due to error: %v", err)
		}
	}()

	test := []struct {
		Name      string
		Method    string
		Failed    bool
		Endpoint  string
		AuthToken string
		Body      map[string]string
	}{
		{
			Name:      "Trying to give to the code my own github (they won't crack it, I hope)",
			Method:    "POST",
			Failed:    false,
			Endpoint:  "http://127.0.0.1:8080/oauth/github/callback/mock",
			AuthToken: "Bearer 00000000000000000000000000000001",
			Body: map[string]string{
				"token": githubToken,
			},
		},
		{
			Name:      "Failing due to empty token payload",
			Method:    "POST",
			Failed:    true,
			Endpoint:  "http://127.0.0.1:8080/oauth/github/callback/mock",
			AuthToken: "Bearer 00000000000000000000000000000001",
			Body: map[string]string{
				"token": "",
			},
		},
		{
			Name:      "Giving a fake token that will make everything blow up",
			Method:    "POST",
			Failed:    true,
			Endpoint:  "http://127.0.0.1:8080/oauth/github/callback/mock",
			AuthToken: "Bearer 00000000000000000000000000000001",
			Body: map[string]string{
				"token": "ghp_token_mock",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Post("/oauth/github/callback/mock", h.GitHubCallbackMock)

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			var body io.Reader
			if tt.Body != nil {
				bodyBytes, err := sonic.Marshal(tt.Body)
				if err != nil {
					t.Fatalf("Expected to successfully turn the payload to bytes, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.Method, tt.Endpoint, body)
			w := httptest.NewRecorder()
			req.Header.Set("Authorization", tt.AuthToken)

			r.ServeHTTP(w, req)

			if tt.Failed {
				if w.Code == 200 {
					t.Errorf("Expected to fail, but got status code 200, and response: %s", w.Body.String())
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected to succeed, but got code: %d, and response: %s", w.Code, w.Body.String())
			}
		})
	}
}
