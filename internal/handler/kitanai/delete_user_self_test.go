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
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/lib"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func TestDeleteUserSelf(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	DATABASE_URL := os.Getenv("DATABASE_URL")
	if DATABASE_URL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	REDIS_ADDR := os.Getenv("REDIS_ADDR")
	if REDIS_ADDR == "" {
		t.Fatal("Missing 'REDIS_ADDR' inside .env")
	}
	REDIS_PASS := os.Getenv("REDIS_PASSWORD")
	if REDIS_PASS == "" {
		t.Fatal("Missing 'REDIS_PASSWORD'")
	}

	config, err := pgxpool.ParseConfig(DATABASE_URL)
	if err != nil {
		t.Fatalf("Expected to get the pgxpool configuration, but got: %v", err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeout, config)
	if err != nil {
		t.Fatalf("Expected to get the conncection pool, but got: %v", err)
	}

	queries := db.New(pool)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     REDIS_ADDR,
		Password: REDIS_PASS,
		DB:       0,
	})

	workers := kitanaijob.SetupDeletionWorkers(queries)
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("Expected to create a new river client, but got: %v", err)
	}

	h := kitanai.New(pool, queries, nil, nil, redisClient, riverClient)

	auth.Init(queries, redisClient)

	err = h.Queries.TruncateEverythingBeforeTest(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully delete the users and the sessions, due to error: %v", err)
	}
	err = h.Queries.InsertDummiesInsideUsers(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the database due to this error: %v", err)
	}
	err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+REDIS_PASS, "REDIS_PATH=internal/handler/testdata/create_session.redis")
	if err != nil {
		t.Fatalf("Couldn't successfully create mock session token, due to error: %v", err)
	}
	defer func() {
		err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+REDIS_PASS, "REDIS_PATH=internal/handler/testdata/clean_session.redis")
		if err != nil {
			t.Fatalf("Couldn't successfully clean mock session token, due to error: %v", err)
		}
	}()

	test := []struct {
		Name      string
		Method    string
		Endpoint  string
		AuthToken string
		Failed    bool
		Body      map[string]string
	}{
		{
			Name:      "Trying to throw a self-deletion declaration as Sheena Totsuki",
			Method:    "DELETE",
			Endpoint:  "http://127.0.0.1:8080/users/delete/self",
			AuthToken: "Bearer 00000000000000000000000000000002",
			Failed:    false,
			Body: map[string]string{
				"status":        "CONFIRM",
				"clarification": "The application sucks",
			},
		},
		{
			Name:      "Forgot to confirm the deletion",
			Method:    "DELETE",
			Endpoint:  "http://127.0.0.1:8080/users/delete/self",
			AuthToken: "Bearer 00000000000000000000000000000002",
			Failed:    true,
			Body: map[string]string{
				"clarification": "*gawking*",
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Delete("/users/delete/self", h.DeleteUserSelf)

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			var body io.Reader
			if tt.Body != nil {
				bodyBytes, err := sonic.Marshal(tt.Body)
				if err != nil {
					t.Fatalf("Tried to decode the given body, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.Method, tt.Endpoint, body)
			w := httptest.NewRecorder()
			req.Header.Set("Authorization", tt.AuthToken)

			r.ServeHTTP(w, req)

			if tt.Failed {
				if w.Code == 200 {
					t.Error("Expected failure but got status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected status 200; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
