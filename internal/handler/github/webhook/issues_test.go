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

func TestWebhookIssues(t *testing.T) {
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

	err = h.Queries.TruncateEverythingBeforeTest(timeout)
	if err != nil {
		t.Fatalf("Expected to run the sql command and clean everything about users, but got: %v", err)
	}
	err = h.Queries.DeleteGitHubMock(timeout)
	if err != nil {
		t.Fatalf("Expected to run the sql command and clean everything about github, but got: %v", err)
	}
	err = h.Queries.InsertDummiesInsideUsers(timeout)
	if err != nil {
		t.Fatalf("Expected to run the sql command and create dummies, but got: %v", err)
	}
	err = h.Queries.InsertDummyGitHubUsers(timeout)
	if err != nil {
		t.Fatalf("Expected to run the sql command and create dummies related to github users, though got: %v", err)
	}
	err = h.Queries.InsertDummyRepositories(timeout)
	if err != nil {
		t.Fatalf("Expected to run the sql command and create dummies related to github repositories, though got: %v", err)
	}
	err = lib.RunMake("../../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/create_github_fluff.redis")
	if err != nil {
		t.Fatalf("Expected to run the 'make' command and create the github fluff, but got: %v", err)
	}
	defer func() {
		err = h.Queries.TruncateEverythingBeforeTest(timeout)
		if err != nil {
			t.Fatalf("Expected to run the sql command and clean everything, but got: %v", err)
		}
		err = lib.RunMake("../../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/clean_github_fluff.redis")
		if err != nil {
			t.Fatalf("Expected to run the 'make' command and create the github fluff, but got: %v", err)
		}
	}()

	healthyStr := func(str string) *string {
		return &str
	}

	// The following "test" was made with the help of AI
	test := []struct {
		Name     string
		Method   string
		Endpoint string
		Failed   bool
		Body     *webhook.GitHubIssues
	}{
		{
			Name:     "healthy issue",
			Method:   http.MethodPost,
			Endpoint: "/oauth/github/webhook",
			Failed:   false,
			Body: &webhook.GitHubIssues{
				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "test/repository",
					OwnerID: webhook.Owner{
						ID: 10000001,
					},
				},
				Action: "assigned",
				AssigneeInfo: &webhook.Assignee{
					ID:   10000002,
					Name: "developer",
					Type: "User",
				},
				IssueInfo: webhook.Issue{
					ID:              900001,
					AuthAssociation: "OWNER",
					Body:            healthyStr("This is a valid issue."),
					Comments:        2,
					Draft:           false,
					CreatedAt:       time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
					Labels: []webhook.Label{
						{
							ID:          500001,
							Name:        "bug",
							Description: healthyStr("Something is not working."),
						},
					},
					Locked: false,
					Number: 1,
					Reactions: webhook.Reactions{
						TotalCount:       3,
						PositiveReaction: 2,
						NegativeReaction: 1,
					},
					State: "open",
					SubIssuesSummary: webhook.SubIssuesSummary{
						Total:            2,
						Completed:        1,
						PercentCompleted: 50,
					},
					IssueDependenciesSummary: webhook.IssueDependenciesSummary{
						TotalBlockedBy: 0,
						TotalBlocking:  1,
					},
					Title:     "Fix authentication bug",
					UpdatedAt: time.Date(2026, 10, 4, 11, 0, 0, 0, time.UTC),
					User: &webhook.User{
						ID:   10000001,
						Name: "owner",
						Type: "User",
					},
				},
				Sender: webhook.Sender{
					ID:    10000001,
					Login: "Mimi Kagari",
					Type:  "Owner",
				},
			},
		},
		{
			Name:     "invalid issue state",
			Method:   http.MethodPost,
			Endpoint: "/oauth/github/webhook",
			Failed:   true,
			Body: &webhook.GitHubIssues{
				RepoInfo: webhook.Repository{
					ID:       5839202,
					FullName: "test/repository",
					OwnerID: webhook.Owner{
						ID: 10000006,
					},
				},
				Action: "assigned",
				AssigneeInfo: &webhook.Assignee{
					ID:   10000002,
					Name: "developer",
					Type: "User",
				},
				IssueInfo: webhook.Issue{
					ID:        900002,
					Title:     "Invalid state issue",
					State:     "pending",
					Comments:  0,
					Draft:     false,
					CreatedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
				},
				Sender: webhook.Sender{
					ID:    1024333,
					Login: "Ari Maud",
					Type:  "User",
				},
			},
		},
		{
			Name:     "missing issue title",
			Method:   http.MethodPost,
			Endpoint: "/oauth/github/webhook",
			Failed:   true,
			Body: &webhook.GitHubIssues{
				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "test/repository",
					OwnerID: webhook.Owner{
						ID: 10000001,
					},
				},
				Action: "assigned",
				AssigneeInfo: &webhook.Assignee{
					ID:   10000002,
					Name: "developer",
					Type: "User",
				},
				IssueInfo: webhook.Issue{
					ID:        900003,
					Title:     "",
					State:     "open",
					Comments:  0,
					Draft:     false,
					CreatedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
				},
				Sender: webhook.Sender{
					ID:    3283923,
					Login: "Lizzy Seiran",
					Type:  "User",
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
					t.Fatalf("Expected to turn the given body in bytes, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.Method, tt.Endpoint, body)
			w := httptest.NewRecorder()

			repoIDStr := strconv.FormatInt(tt.Body.RepoInfo.ID, 10)
			secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
			if err != nil {
				t.Fatalf("Expected to successfully retrieve the secret, but got: %v", err)
			}
			hash := hmac.New(sha256.New, []byte(secret))
			hash.Write(bodyBytes)
			fullHash := hex.EncodeToString(hash.Sum(nil))
			req.Header.Set("X-Hub-Signature-256", "sha256="+fullHash)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Delivery", "otomachi_una_my_daughter")
			req.Header.Set("X-GitHub-Event", "issues")

			r.ServeHTTP(w, req)

			if tt.Failed {
				if w.Code == 200 {
					t.Error("expected to fail, but ended up getting status code 200")
				}
				return
			}

			if w.Code != 200 {
				t.Fatalf("Expected to succeed, but ended up getting status code %d, and response: %s", w.Code, w.Body.String())
			}
		})
	}
}
