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
	"prolepsis/internal/lib"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestUpdateUserInfo(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	email := os.Getenv("FROM_EMAIL")
	if email == "" {
		t.Fatal("Missing 'FROM_EMAIL' inside .env")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("Missing 'redisAddr' inside .env")
	}
	redisPSWD := os.Getenv("REDIS_PASSWORD")
	if redisPSWD == "" {
		t.Fatal("Missing 'REDIS_PASSWORD' inside .env")
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
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPSWD,
		DB:       0,
	})

	h := kitanai.New(nil, queries, nil, nil, redisClient, nil)
	auth.Init(queries, redisClient)

	err = h.Queries.TruncateEverythingBeforeTest(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully delete the users and the sessions, due to error: %v", err)
	}
	err = h.Queries.InsertDummiesInsideUsers(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the database due to this error: %v", err)
	}
	err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/create_session.redis")
	if err != nil {
		t.Fatalf("Couldn't successfully create mock session token, due to error: %v", err)
	}
	defer func() {
		err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/clean_session.redis")
		if err != nil {
			t.Fatalf("Couldn't successfully clean mock session token, due to error: %v", err)
		}
	}()

	test := []struct {
		name      string
		method    string
		endpoint  string
		authToken string
		failed    bool
		body      map[string]any
	}{
		{
			// The "UpdateUserInfo" function is hard-locked to the user who sent the request (meaning that nobody could update something of somebody else)
			name:      "Updating Ethan Winter's display_name, bio, location",
			method:    "PATCH",
			endpoint:  "/users/update",
			authToken: "Bearer 00000000000000000000000000000015",
			failed:    false,
			body: map[string]any{
				"display_name": "Rosemarys_Father",
				"bio":          "A young system engineer, and the best father in game history",
				"location":     "US, Louisiana",
			},
		},
		{
			name:      "Failing due to misconfigured location",
			method:    "PATCH",
			endpoint:  "/users/update",
			authToken: "Bearer 00000000000000000000000000000018",
			failed:    true,
			body: map[string]any{
				"display_name": "Shrine_Maiden",
				"bio":          "A young layabout who loves sleeping and doing nothing",
				"location":     "Gensokyo",
			},
		},
		{
			name:      "Failing due to empty body",
			method:    "PATCH",
			endpoint:  "/users/update",
			authToken: "Bearer 00000000000000000000000000000022",
			failed:    true,
			body:      nil,
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Patch("/users/update", h.UpdateUserInfo)

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
			req.Header.Set("Authorization", tt.authToken)

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 200 {
					t.Error("Expected failure, but got status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected status 200; got status %d \nResponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
