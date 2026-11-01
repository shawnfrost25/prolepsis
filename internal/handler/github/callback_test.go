package oauthGithub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type MyTrip struct {
	Reason string
}

type resp struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// What I used as reference
/*
--- Inside callback.go:
token, err := h.GH_Config.Exchange(timeout, code)

--- Inside token.go:
var token *Token
content, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
switch content {
case "application/x-www-form-urlencoded", "text/plain":

		///

return &Token{
	AccessToken:  t.AccessToken,
	TokenType:    t.TokenType,
	RefreshToken: t.RefreshToken,
	Expiry:       t.Expiry,
	ExpiresIn:    t.ExpiresIn,
	raw:          t.Raw,
}

--- Inside oauth2.go:
v := url.Values{
	"grant_type": {"authorization_code"},
	"code":       {code},
}
if c.RedirectURL != "" {
	v.Set("redirect_uri", c.RedirectURL)
}

--- Inside client.go:
if resp != nil && resp.Request != nil {
	urlStr = stripPassword(resp.Request.URL)
} else {
	urlStr = stripPassword(req.URL)
}
*/
func Exchange(req *http.Request) (*http.Response, error) {
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse 'Content-Type': %v", err)
	}

	if mediaType != "application/x-www-form-urlencoded" {
		return nil, fmt.Errorf("Unexpected 'Content-Type': \nProvided type: %v \nExpected type: application/x-www-form-urlencoded", mediaType)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	code := values.Get("code")
	if code != "sheena12345" {
		return nil, fmt.Errorf("Unexpected code: \nProvided code: %s \nExpected code: sheena12345", code)
	}

	redirectUri := values.Get("redirect_uri")
	if redirectUri != "http://127.0.0.1:8080/oauth/github/callback" {
		return nil, fmt.Errorf("Unexpected redirect uri: \nProvided uri: %s \nExpected uri: http://127.0.0.1:8080/oauth/github/callback", redirectUri)
	}

	bytesBody, err := json.Marshal(resp{
		AccessToken:  "mimi-access-token-123",
		TokenType:    "Bearer",
		RefreshToken: "mock-refresh-token-456",
		ExpiresIn:    3600,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal token response: %w", err)
	}

	b := io.NopCloser(bytes.NewReader(bytesBody))

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          b,
		ContentLength: int64(len(bytesBody)),
		Request:       req,
	}, nil
}

// What I used as reference
/*
reqGraphQl, err := http.NewRequestWithContext(timeout, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(bodyGraphQl))
if err != nil {
	err = failure.Wrap("github.http.graphql.request.create", err)
	errlog.General(logger, w, "graphql_request_create", failure.InternalGeneralError, err, trace)
	return
}
reqGraphQl.Header.Set("Authorization", "Bearer "+token.AccessToken)
reqGraphQl.Header.Set("Content-Type", "application/json")
reqGraphQl.Header.Set("User-Agent", "Prolepsis-IWTLYTYDD")

respGraphQl, err := h.HTTP_Client.Do(reqGraphQl)
if err != nil {
	err = failure.Wrap("github.http.graphql.request.send", err)
	errlog.General(logger, w, "graphql_request_send", failure.InternalGeneralError, err, trace)
	return
}
*/
func (t *MyTrip) GraphQL(req *http.Request) (*http.Response, error) {
	parts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)

	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("Provided token is malformed: \nLength: %d \nContent: %v", len(parts), parts)
	}

	agent := req.Header.Get("User-Agent")
	if agent != "Prolepsis-IWTLYTYDD" {
		return nil, fmt.Errorf("Missmatched user agent: \nProvided agent: %s \nExpected agent: Prolepsis-IWTLYTYDD", agent)
	}

	content := req.Header.Get("Content-Type")
	if content != "application/json" {
		return nil, fmt.Errorf("Missmatched content type: \nProvided type: %s \nExpected type: application/json", content)
	}

	// Since it's http.Do - we have full control of what we send and give
	body, err := os.ReadFile("testdata/graphql.json")
	if err != nil {
		return nil, fmt.Errorf("Failed to read Provided body: %v", err)
	}

	if t.Reason == "graphql_invalid" {
		var payload map[string]any
		err := sonic.Unmarshal(body, &payload)
		if err != nil {
			return nil, fmt.Errorf("Fauled to parse given body: %v", err)
		}

		data := payload["data"].(map[string]any)
		viewer := data["viewer"].(map[string]any)
		// Missing id
		viewer["databaseId"] = nil

		body, err = sonic.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GraphQL response: %w", err)
		}
	}

	b := io.NopCloser(bytes.NewReader(body))

	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       b,
	}, nil
}

// What I used as reference
/*
webhookURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", graph.Login, repo.Name)

reqW, err := http.NewRequestWithContext(timeout, http.MethodPost, webhookURL, body)
if err != nil {
	err = failure.Wrap("github.http.webhook.request.create", err)
	logger.Debug().
		Err(err).
		Int("status", http.StatusInternalServerError).
		Str("code", "webhook_request_create_failed").
		Msg("failed to create webhook request")
	continue
}

reqW.Header.Set("Accept", "application/vnd.github.json")
reqW.Header.Set("Authorization", "Bearer "+token.AccessToken)
reqW.Header.Set("X-GitHub-Api-Version", "2026-03-10")
*/
func (t *MyTrip) Webhook(req *http.Request) (*http.Response, error) {
	v := req.Header.Get("X-GitHub-Api-Version")
	if v != "2026-03-10" {
		return nil, fmt.Errorf("Mismatched version: \nProvided version: %s \nExpected version: 2026-03-10", v)
	}

	parts := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("Provided token is malformed: \nLength: %d \nContent: %v", len(parts), parts)
	}

	accept := req.Header.Get("Accept")
	if accept != "application/vnd.github.json" {
		return nil, fmt.Errorf("Invalid type: \nProvided Type: %s \nExpected type: application/vnd.github.json", accept)
	}

	var s GitHubWebhookCreation
	err := sonic.ConfigDefault.NewDecoder(req.Body).Decode(&s)
	if err != nil {
		return nil, err
	}

	if t.Reason == "webhook_invalid" {
		s.Config.ContentType = "mock_type"
	}

	if s.Config.ContentType != "json" {
		return nil, fmt.Errorf("Invalid type: \nProvided type: %s \nExpected type: json", s.Config.ContentType)
	}

	if len(s.Config.Secret) != 64 {
		return nil, fmt.Errorf("Invalid secret length: \nLength: %d \nSecret: %s", len(s.Config.Secret), s.Config.Secret)
	}

	body, err := os.ReadFile("testdata/webhook_create_response.json")
	if err != nil {
		return nil, err
	}

	b := io.NopCloser(bytes.NewReader(body))

	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       b,
	}, nil
}

// Distribute based on url to match RoundTrip
func (t *MyTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token":
		return Exchange(req)
	case req.URL.Host == "api.github.com" && strings.HasSuffix(req.URL.Path, "/graphql"):
		return t.GraphQL(req)
	case req.URL.Host == "api.github.com" && strings.HasSuffix(req.URL.Path, "/hooks"):
		return t.Webhook(req)
	default:
		return nil, fmt.Errorf("Unverified request provided \nProvided url: %s", req.URL)
	}
}

func cb(t *testing.T, trip *MyTrip) *Server {
	t.Helper()
	rdb, pool, queries := testenv.New(t)
	_ = godotenv.Load("../../../.env")
	clientID := os.Getenv("CLIENT_ID")
	if clientID == "" {
		t.Fatalf("Missing 'CLIENT_ID' inside .env")
	}
	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientSecret == "" {
		t.Fatalf("Missing 'CLIENT_SECRET' inside .env")
	}

	oauthConf := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  "http://127.0.0.1:8080/oauth/github/callback",
		Scopes:       []string{"read:user", "repo"},
	}

	httpClient := http.Client{
		Transport: trip,
	}

	h := GH_New(rdb, oauthConf, httpClient, queries)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Logger_Middleware)
		r.Post("/oauth/github/callback", h.GitHubCallback)
	})

	return ServerNew(r, rdb, pool, queries)
}

func TestCallback(t *testing.T) {
	test := []struct {
		Name           string
		Endpoint       string
		Reason         string
		ExpectedStatus int
	}{
		{
			Name:           "Successfully passing callback",
			Endpoint:       "/oauth/github/callback?code=sheena12345&state=mimi_ga_mimi_dayo",
			Reason:         "",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Failing due to invalid code",
			Endpoint:       "/oauth/github/callback?code=mimi12345&state=mimi_ga_mimi_dayo",
			Reason:         "",
			ExpectedStatus: http.StatusInternalServerError,
		},
		{
			Name:           "Failing due to invalid state",
			Endpoint:       "/oauth/github/callback?code=sheena12345&state=sheena_totsuki",
			Reason:         "",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Failing due to invalid webhook creation payload",
			Endpoint:       "/oauth/github/callback?code=sheena12345&state=mimi_ga_mimi_dayo",
			Reason:         "webhook_invalid",
			ExpectedStatus: http.StatusInternalServerError,
		},
		{
			Name:           "Failing due to invalid GraphQL body",
			Endpoint:       "/oauth/github/callback?code=sheena12345&state=mimi_ga_mimi_dayo",
			Reason:         "graphql_invalid",
			ExpectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			var s *Server
			if tt.Reason == "" {
				s = cb(t, &MyTrip{})
			}
			if tt.Reason != "" {
				s = cb(t, &MyTrip{
					Reason: tt.Reason,
				})
			}

			mimiID := s.seed(t, "Mimi Kagari", "")
			_, err := s.redis.Set(context.Background(), "oauth:github:state:mimi_ga_mimi_dayo", mimiID.String(), 10*time.Minute).Result()
			if err != nil {
				t.Fatalf("Failed to insert state inside redis: %v", err)
			}

			w := s.do(t, http.MethodPost, tt.Endpoint, "", nil)

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
