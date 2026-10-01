package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	oauthGithub "prolepsis/internal/handler/github"
	"prolepsis/internal/handler/github/webhook"
	"prolepsis/internal/lib"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

func TestWebhookPush(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'databaseURL' inside .env")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("Missing 'REDIS_ADDR' inside .env")
	}
	redisPSWD := os.Getenv("REDIS_PASSWORD")
	if redisPSWD == "" {
		t.Fatal("Missing 'REDIS_PASSWORD' inside .env")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("Expected to create a pool configuration, but got: %v", err)
	}
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeout, config)
	if err != nil {
		t.Fatalf("Expected to create a pool connection, but got: %v", err)
	}

	queries := db.New(pool)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPSWD,
		DB:       0,
	})

	h := oauthGithub.GH_New(redisClient, oauth2.Config{}, http.Client{}, queries)

	err = lib.RunMake("../../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/create_github_fluff.redis")
	if err != nil {
		t.Fatalf("Expected to run the 'make' command and create the github fluff, but got: %v", err)
	}
	defer func() {
		err = lib.RunMake("../../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/clean_github_fluff.redis")
		if err != nil {
			t.Fatalf("Expected to run the 'make' command and create the github fluff, but got: %v", err)
		}
	}()

	// Made with the help of AI, like, obviously
	test := []struct {
		Name     string
		Method   string
		Endpoint string
		Failed   bool
		Body     *webhook.GitHubPush
	}{
		{
			Name:     "Standard Push to Main",
			Method:   "POST",
			Endpoint: "/oauth/github/webhook",
			Failed:   false,
			Body: &webhook.GitHubPush{
				Ref:       "refs/heads/main",
				BeforeSHA: "a1b2c3d4e5f67890123456789012345678901234",
				AfterSHA:  "f9e8d7c6b5a43210987654321098765432109876",
				Compare:   "https://github.com/myorg/api-service/compare/a1b2c3d4e5f6...f9e8d7c6b5a4",
				Forced:    false,
				Created:   false,
				Deleted:   false,
				HeadCommitID: &webhook.HeadCommit{
					SHA: "f9e8d7c6b5a43210987654321098765432109876",
				},
				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "myorg/api-service",
					OwnerID: webhook.Owner{
						ID: 102938,
					},
				},
				CommitInfo: []webhook.Commit{
					{
						CommitSha:   "b2c3d4e5f6789012345678901234567890123456",
						Message:     "feat(auth): implement JWT token rotation",
						Added:       []string{"pkg/auth/jwt.go", "pkg/auth/jwt_test.go"},
						Removed:     []string{},
						Modified:    []string{"pkg/auth/handler.go"},
						URL:         "https://github.com/myorg/api-service/commit/b2c3d4e5f6789012345678901234567890123456",
						CommittedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
					},
					{
						CommitSha:   "f9e8d7c6b5a43210987654321098765432109876",
						Message:     "fix(config): update database connection pool limits",
						Added:       []string{},
						Removed:     []string{},
						Modified:    []string{"config/production.yaml"},
						URL:         "https://github.com/myorg/api-service/commit/f9e8d7c6b5a43210987654321098765432109876",
						CommittedAt: time.Date(2026, 10, 1, 12, 15, 0, 0, time.UTC),
					},
				},
			},
		},
		{
			Name:     "Force Push to Feature Branch",
			Method:   "POST",
			Endpoint: "/oauth/github/webhook",
			Failed:   false,
			Body: &webhook.GitHubPush{
				Ref:       "refs/heads/feature/oauth2-login",
				BeforeSHA: "1111222233334444555566667777888899990000",
				AfterSHA:  "9999888877776666555544443333222211110000",
				Compare:   "https://github.com/myorg/api-service/compare/111122223333...999988887777",
				Forced:    true,
				Created:   false,
				Deleted:   false,
				HeadCommitID: &webhook.HeadCommit{
					SHA: "9999888877776666555544443333222211110000",
				},
				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "myorg/api-service",
					OwnerID: webhook.Owner{
						ID: 102938,
					},
				},
				CommitInfo: []webhook.Commit{
					{
						CommitSha:   "9999888877776666555544443333222211110000",
						Message:     "squash! feat: rebase oauth2 changes onto main",
						Added:       []string{"pkg/oauth/provider.go"},
						Removed:     []string{"pkg/oauth/deprecated.go"},
						Modified:    []string{"go.mod", "go.sum"},
						URL:         "https://github.com/myorg/api-service/commit/9999888877776666555544443333222211110000",
						CommittedAt: time.Date(2026, 10, 1, 13, 30, 0, 0, time.UTC),
					},
				},
			},
		},
		{
			Name:     "Failing due to messy sha",
			Method:   "POST",
			Endpoint: "/oauth/github/webhook",
			Failed:   true,
			Body: &webhook.GitHubPush{
				Ref:       "refs/heads/hotfix/v1.2.1-patch",
				BeforeSHA: "0000000000000000000000000000000000000000",
				AfterSHA:  "c7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6",
				Compare:   "https://github.com/myorg/api-service/commit/c7a8b9c0d1e2",
				Forced:    false,
				Created:   true,
				Deleted:   false,
				HeadCommitID: &webhook.HeadCommit{
					SHA: "c7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6",
				},
				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "myorg/api-service",
					OwnerID: webhook.Owner{
						ID: 102938,
					},
				},
				CommitInfo: []webhook.Commit{
					{
						CommitSha:   "c7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6",
						Message:     "hotfix: resolve nil pointer exception in webhook handler",
						Added:       []string{},
						Removed:     []string{},
						Modified:    []string{"pkg/webhook/handler.go"},
						URL:         "https://github.com/myorg/api-service/commit/c7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6",
						CommittedAt: time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Logger_Middleware)
	r.Post("/oauth/github/webhook", h.GitHubWebhook)

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			var body io.Reader
			var bodyBytes []byte
			if tt.Body != nil {
				bodyBytes, err = sonic.Marshal(tt.Body)
				if err != nil {
					t.Fatalf("Tried to marshal the given body, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			repoIDStr := strconv.FormatInt(tt.Body.RepoInfo.ID, 10)
			secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
			if err != nil {
				t.Fatalf("Tried to get the webhook secret, but got: %v", err)
			}
			req := httptest.NewRequest(tt.Method, tt.Endpoint, body)
			hash := hmac.New(sha256.New, []byte(secret))
			hash.Write(bodyBytes)
			fullHash := hex.EncodeToString(hash.Sum(nil))
			req.Header.Set("X-Hub-Signature-256", "sha256="+fullHash)
			if tt.Failed {
				// missing "sha256=" prefix, which will trigger an error
				req.Header.Set("X-Hub-Signature-256", fullHash)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Delivery", "otomachi_una_my_daughter")
			req.Header.Set("X-GitHub-Event", "push")

			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.Failed {
				if w.Code == 200 {
					t.Error("Expected to fail, but got status code 200")
				}
				return
			}

			if w.Code != 200 {
				t.Fatalf("Expected to succeed, but ended up getting status code %d and response: \n%s", w.Code, w.Body.String())
			}
		})
	}
}
