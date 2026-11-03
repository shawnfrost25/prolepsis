package oauthGithub

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/testenv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
)

func w(t *testing.T) *Server {
	rdb, pool, queries := testenv.New(t)

	h := GH_New(rdb, oauth2.Config{}, *http.DefaultClient, queries)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Logger_Middleware)
		r.Post("/oauth/github/webhook", h.GitHubWebhook)
	})

	return ServerNew(r, rdb, pool, queries)
}

func TestGitHubWebhook(t *testing.T) {
	s := w(t)
	ctx := context.Background()

	ptrStr := func(str string) *string {
		return &str
	}

	mimiID := s.seed(t, "Mimi Kagari", "")

	err := s.queries.InsertGitHubUser(ctx, db.InsertGitHubUserParams{
		UserID:        mimiID,
		GithubID:      12345,
		Login:         "mimikagari",
		Company:       ptrStr("MimiAndSheenaComp"),
		Email:         ptrStr("mimikagari@gmail.com"),
		Bio:           ptrStr("Cookie and Creams"),
		StatusMessage: ptrStr("MIMI GA MIMI DAYO"),
		Followers:     99999,
		TotalRepos:    1,
		IsHireable:    false,
		AvatarUrl:     "mimiURL",
		HtmlUrl:       "mimiURL",
		CreatedAt:     pgtype.Timestamptz{Time: time.Date(2000, 12, 03, 1, 1, 1, 1, time.UTC), Valid: true},
	})

	if err != nil {
		t.Fatalf("Failed github user insertion: %v", err)
	}

	err = s.queries.InsertGitHubRepo(ctx, db.InsertGitHubRepoParams{
		GithubID:           12345,
		RepoID:             54321,
		HookID:             98765,
		Action:             ptrStr("created"),
		Name:               "mimikagari-repo",
		Description:        ptrStr("A repository for Mimi"),
		Readme:             ptrStr("# Mimi's Repo\nWelcome!"),
		MainLanguage:       ptrStr("Go"),
		SecondaryLanguages: []string{"Rust", "TypeScript"},
		LicenseKey:         nil,
		Topics:             []string{"mimi", "cookies", "cream"},
		Visibility:         "public",
		Stars:              100,
		Watches:            50,
		ForksCount:         10,
		TotalReleases:      5,
		IsArchived:         false,
		HtmlUrl:            "https://github.com/mimikagari/mimikagari-repo",
		CreatedAt:          pgtype.Timestamptz{Time: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC), Valid: true},
		PushedAt:           pgtype.Timestamptz{Time: time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC), Valid: true},
		UpdatedAt:          pgtype.Timestamptz{Time: time.Date(2023, 1, 3, 12, 0, 0, 0, time.UTC), Valid: true},
	})

	if err != nil {
		t.Fatalf("Failed github repository insertion: %v", err)
	}

	push, err := os.ReadFile("testdata/webhook_push.json")
	if err != nil {
		t.Fatalf("Failed to read push json: %v", err)
	}

	pullRequest, err := os.ReadFile("testdata/webhook_pull_request.json")
	if err != nil {
		t.Fatalf("Failed to read pull_request json: %v", err)
	}

	issue, err := os.ReadFile("testdata/webhook_issue.json")
	if err != nil {
		t.Fatalf("Failed to read issue json: %v", err)
	}

	issueComment, err := os.ReadFile("testdata/webhook_issue_comment.json")
	if err != nil {
		t.Fatalf("Failed to read issue_comment json: %v", err)
	}

	releases, err := os.ReadFile("testdata/webhook_releases.json")
	if err != nil {
		t.Fatalf("Failed to read releases json: %v", err)
	}

	repository, err := os.ReadFile("testdata/webhook_repository.json")
	if err != nil {
		t.Fatalf("Failed to read repository json: %v", err)
	}

	actualSecret := strings.Repeat("sheena12", 8)
	_, err = s.redis.Set(ctx, "oauth:github:webhook:secret:54321", actualSecret, 5*time.Hour).Result()
	if err != nil {
		t.Fatalf("Failed to insert secret inside redis: %v", err)
	}

	createHmac := func(body []byte, req *http.Request) {
		hash := hmac.New(sha256.New, []byte(actualSecret))
		hash.Write(body)
		expected := "sha256=" + hex.EncodeToString(hash.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", expected)
	}

	test := []struct {
		Name           string
		ExpectedStatus int
		Reason         string
		Body           []byte
	}{
		{
			Name:           "Successfully distributing push webhook",
			ExpectedStatus: http.StatusOK,
			Body:           push,
		},
		{
			Name:           "Successfully distributing pull_request webhook",
			ExpectedStatus: http.StatusOK,
			Body:           pullRequest,
		},
		{
			Name:           "Successfully distributing issue webhook",
			ExpectedStatus: http.StatusOK,
			Body:           issue,
		},
		{
			Name:           "Successfully distributing issue_comment webhook",
			ExpectedStatus: http.StatusOK,
			Body:           issueComment,
		},
		{
			Name:           "Successfully distributing releases webhook",
			ExpectedStatus: http.StatusOK,
			Body:           releases,
		},
		{
			Name:           "Successfully distributing repository webhook",
			ExpectedStatus: http.StatusOK,
			Body:           repository,
		},
		{
			Name:           "Failing push due to bad signature",
			ExpectedStatus: http.StatusUnauthorized,
			Reason:         "signature",
			Body:           push,
		},
		{
			Name:           "Failing push due to malformed delivery id",
			ExpectedStatus: http.StatusBadRequest,
			Reason:         "delivery_id",
			Body:           push,
		},
		{
			Name:           "Failing push due to missing type",
			ExpectedStatus: http.StatusBadRequest,
			Reason:         "type",
			Body:           push,
		},
		{
			Name:           "Failing push due to malformed push",
			ExpectedStatus: http.StatusInternalServerError,
			Reason:         "push",
			Body:           push,
		},
		{
			Name:           "Failing pull_request due to invalid pull request state",
			ExpectedStatus: http.StatusBadRequest,
			Reason:         "pull_request",
			Body:           pullRequest,
		},
		{
			Name:           "Successfully updating issue",
			ExpectedStatus: http.StatusOK,
			Reason:         "issue",
			Body:           issue,
		},
		{
			Name:           "Failing issue_comment due to missing issue in",
			ExpectedStatus: http.StatusNotFound,
			Reason:         "issue_comment",
			Body:           issueComment,
		},
		{
			Name:           "Successfully accepting releases deduplicated id",
			ExpectedStatus: http.StatusOK,
			Reason:         "releases",
			Body:           releases,
		},
		{
			Name:           "Failing repository due to missing hook id",
			ExpectedStatus: http.StatusBadRequest,
			Reason:         "repository",
			Body:           repository,
		},
	}

	for i, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {

			if tt.Name == "Successfully distributing repository webhook" {
				_, err := s.redis.Set(ctx, "oauth:github:webhook:hook_id:54321", "98765", 5*time.Hour).Result()
				if err != nil {
					t.Fatalf("failed to seed webhook hook id: %v", err)
				}
			}

			var body io.Reader
			if tt.Body != nil {
				body = bytes.NewReader(tt.Body)
			}

			req := httptest.NewRequest(http.MethodPost, "/oauth/github/webhook", body)

			deliveryID := fmt.Sprintf(
				"d9b23b32-349c-4621-8263-%012x",
				i,
			)
			req.Header.Set("X-GitHub-Delivery", deliveryID)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Hook-ID", "98765")

			var event string
			switch {
			case strings.Contains(tt.Name, "push"):
				event = "push"
			case strings.Contains(tt.Name, "pull_request"):
				event = "pull_request"
			case strings.Contains(tt.Name, "issue_comment"):
				err = s.queries.InsertIssue(ctx, db.InsertIssueParams{
					ID:                 3000000001,
					GithubRepoID:       54321,
					GithubUserID:       12345,
					Action:             ptrStr("opened"),
					AuthorLogin:        ptrStr("shawnfrost25"),
					Title:              "Improve webhook delivery observability",
					Body:               "Track GitHub delivery identifiers and event types for debugging.",
					State:              "open",
					StateReason:        nil,
					IssueType:          nil,
					Labels:             []string{},
					Number:             3,
					ParticipantsNumber: 1,
					PositiveReactions:  0,
					NegativeReactions:  0,
					TotalReactions:     0,
					IsClosed:           false,
					IsLocked:           false,
					HtmlUrl:            "https://github.com/shawnfrost25/prolepsis/issues/3",
					CreatedAt:          pgtype.Timestamptz{Time: time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC), Valid: true},
					UpdatedAt:          pgtype.Timestamptz{Time: time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC), Valid: true},
					ClosedAt:           pgtype.Timestamptz{Valid: false},
				})
				event = "issue_comment"
			case strings.Contains(tt.Name, "issue"):
				event = "issues"
			case strings.Contains(tt.Name, "releases"):
				event = "release"
			case strings.Contains(tt.Name, "repository"):
				event = "repository"
			}
			if event != "" {
				req.Header.Set("X-GitHub-Event", event)
			}

			if tt.Body != nil {
				createHmac(tt.Body, req)
			}

			switch tt.Reason {
			case "signature":
				createHmac([]byte("mimi_ga_mimi_dayo"), req)
			case "delivery_id":
				req.Header.Set("X-GitHub-Delivery", "mimi_ga_mimi_dayo")
			case "type":
				req.Header.Set("Content-Type", "text/plain")
			case "push":
				var b map[string]any
				err := sonic.Unmarshal(tt.Body, &b)
				if err != nil {
					t.Fatalf("Failed to unmarshal push body")
				}
				b["ref"] = 12345
				bodyBytes, err := json.Marshal(b)
				if err != nil {
					t.Fatal("Failed to marshal defected push body")
				}
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				createHmac(bodyBytes, req)
			case "pull_request":
				var b map[string]any
				err := sonic.Unmarshal(tt.Body, &b)
				if err != nil {
					t.Fatalf("Failed to unmarshal push body")
				}
				pr := b["pull_request"].(map[string]any)
				pr["state"] = "mimi_ga_mimi_dayo"
				bodyBytes, err := json.Marshal(b)
				if err != nil {
					t.Fatal("Failed to marshal defected push body")
				}
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				createHmac(bodyBytes, req)
			case "issue":
				err = s.queries.InsertIssue(ctx, db.InsertIssueParams{
					ID:                 3000000001,
					GithubRepoID:       54321,
					GithubUserID:       12345,
					Action:             ptrStr("opened"),
					AuthorLogin:        ptrStr("shawnfrost25"),
					Title:              "Improve webhook delivery observability",
					Body:               "Track GitHub delivery identifiers and event types for debugging.",
					State:              "open",
					StateReason:        nil,
					IssueType:          nil,
					Labels:             []string{},
					Number:             6,
					ParticipantsNumber: 22,
					PositiveReactions:  99999,
					NegativeReactions:  0,
					TotalReactions:     0,
					IsClosed:           false,
					IsLocked:           false,
					HtmlUrl:            "https://github.com/shawnfrost25/prolepsis/issues/3",
					CreatedAt:          pgtype.Timestamptz{Time: time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC), Valid: true},
					UpdatedAt:          pgtype.Timestamptz{Time: time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC), Valid: true},
					ClosedAt:           pgtype.Timestamptz{Valid: false},
				})
				if err != nil {
					t.Fatalf("Failed github issue insertion: %v", err)
				}
			case "issue_comment":
				var b map[string]any
				if err := sonic.Unmarshal(tt.Body, &b); err != nil {
					t.Fatal(err)
				}
				issuePayload := b["issue"].(map[string]any)
				issuePayload["id"] = float64(9999999999)
				badBody, err := json.Marshal(b)
				if err != nil {
					t.Fatal(err)
				}
				req.Body = io.NopCloser(bytes.NewReader(badBody))
				createHmac(badBody, req)
			case "releases":
				var pg pgtype.UUID
				err := pg.Scan(deliveryID)
				if err != nil {
					t.Fatalf("Failed to scan delivery id: %v", err)
				}
				_, err = s.queries.InsertWebhookDeliveries(ctx, pg)
				if err != nil {
					t.Fatalf("Failed to insert webhook delivery id: %v", err)
				}
			case "repository":
				_, err := s.redis.Set(ctx, "oauth:github:webhook:hook_id:54321", "", 5*time.Hour).Result()
				if err != nil {
					t.Fatalf("failed to seed empty webhook hook id: %v", err)
				}
			}

			rec := httptest.NewRecorder()

			s.router.ServeHTTP(rec, req)

			if rec.Code != tt.ExpectedStatus {
				t.Errorf("\nProvided status: %d \nExpected status: %d \nReason: %s \nResponse: %s", rec.Code, tt.ExpectedStatus, tt.Reason, rec.Body.String())
			}
		})
	}
}
