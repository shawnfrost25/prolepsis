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

func TestWebhookPullRequest(t *testing.T) {
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

	healthyBool := func(anyBool bool) *bool {
		return &anyBool
	}
	healthyInt64 := func(num int64) *int64 {
		return &num
	}

	test := []struct {
		Name     string
		Method   string
		Endpoint string
		Failed   bool
		Body     *webhook.GitHubPullRequest
	}{
		{
			Name:     "healthy pull request",
			Method:   http.MethodPost,
			Endpoint: "/oauth/github/webhook",
			Failed:   false,
			Body: &webhook.GitHubPullRequest{
				EventAction: "assigned",

				RepoInfo: webhook.Repository{
					ID:       5839201,
					FullName: "octocat/Hello-World",
					OwnerID: webhook.Owner{
						ID: 10000001,
					},
				},

				SentBy: webhook.Sender{
					ID:    1,
					Login: "octocat",
					Type:  "User",
				},

				PRInfo: webhook.PullRequestInfo{
					ID:       1,
					Number:   1347,
					Title:    "Amazing new feature",
					State:    "open",
					IsDraft:  false,
					IsMerged: healthyBool(false),
					Auth: &webhook.Author{
						ID:    1,
						Login: "octocat",
						Type:  "User",
					},
				},

				TimeStamp: webhook.TimeStamp{
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					ClosedAt:  nil,
					MergedAt:  nil,
				},

				Metrics: webhook.Metrics{
					Additions:           100,
					Deletions:           3,
					ChangedFiles:        5,
					CommitsCount:        3,
					CommentsCount:       10,
					ReviewCommentsCount: 0,
				},

				GitInfo: webhook.Git{
					HeadBranch:     "new-topic",
					HeadSha:        "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					BaseBranch:     "master",
					MergeCommitSha: nil,
				},

				Workflow: webhook.Workflow{
					AssigneeIDs:          []int64{1},
					RequestedReviewerIDs: []int64{},
					RequestedTeamIDs:     []int64{},
					Labels:               []string{"bug"},
					MilestoneID:          healthyInt64(1002604),
				},
			},
		},

		{
			Name:     "defected pull request",
			Method:   http.MethodPost,
			Endpoint: "/oauth/github/webhook",
			Failed:   true,
			Body: &webhook.GitHubPullRequest{
				EventAction: "assigned",

				RepoInfo: webhook.Repository{
					ID:       5839202,
					FullName: "octocat/Defected-Repo",
					OwnerID: webhook.Owner{
						ID: 10000006,
					},
				},

				SentBy: webhook.Sender{
					ID:    1,
					Login: "octocat",
					Type:  "User",
				},

				PRInfo: webhook.PullRequestInfo{
					ID:       2,
					Number:   42,
					Title:    "Broken feature",
					State:    "defected",
					IsDraft:  false,
					IsMerged: healthyBool(false),
					Auth: &webhook.Author{
						ID:    1,
						Login: "octocat",
						Type:  "User",
					},
				},

				TimeStamp: webhook.TimeStamp{
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					ClosedAt:  nil,
					MergedAt:  nil,
				},

				Metrics: webhook.Metrics{
					Additions:           20,
					Deletions:           5,
					ChangedFiles:        2,
					CommitsCount:        1,
					CommentsCount:       0,
					ReviewCommentsCount: 0,
				},

				GitInfo: webhook.Git{
					HeadBranch:     "broken-branch",
					HeadSha:        "abcdef1234567890",
					BaseBranch:     "master",
					MergeCommitSha: nil,
				},

				Workflow: webhook.Workflow{
					AssigneeIDs:          []int64{1},
					RequestedReviewerIDs: []int64{},
					RequestedTeamIDs:     []int64{},
					Labels:               []string{},
					MilestoneID:          nil,
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
					t.Fatalf("Expected to marshal the incoming body, but got: %v", err)
				}
				body = bytes.NewReader(bodyBytes)
			}

			req := httptest.NewRequest(tt.Method, tt.Endpoint, body)
			w := httptest.NewRecorder()

			repoIDStr := strconv.FormatInt(tt.Body.RepoInfo.ID, 10)
			secret, err := h.RedisClient.Get(timeout, "oauth:github:webhook:secret:"+repoIDStr).Result()
			if err != nil {
				t.Fatalf("Tried to get the webhook secret, but got: %v", err)
			}
			hash := hmac.New(sha256.New, []byte(secret))
			hash.Write(bodyBytes)
			fullHash := hex.EncodeToString(hash.Sum(nil))
			req.Header.Set("X-Hub-Signature-256", "sha256="+fullHash)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Delivery", "otomachi_una_my_daughter")
			req.Header.Set("X-GitHub-Event", "pull_request")

			r.ServeHTTP(w, req)

			if tt.Failed {
				if w.Code == 200 {
					t.Error("expected to fail, but ended up getting status code 200")
				}
				return
			}

			if w.Code != 200 {
				t.Fatalf("Expected to succeed, but ended up getting status code %d and response: \n%s", w.Code, w.Body.String())
			}
		})
	}
}
