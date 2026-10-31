package oauthGithub

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"prolepsis/internal/auth"
	"prolepsis/internal/testenv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func r(t *testing.T) *Server {
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

	oauthConfig := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  "http://127.0.0.1:8080/oauth/github/callback",
		Scopes:       []string{"read:user", "repo"},
	}

	httpC := http.Client{
		Timeout: 1 * time.Minute,
	}

	h := GH_New(rdb, oauthConfig, httpC, queries)
	auth.Init(queries, rdb)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Use(auth.Logger_Middleware)
		r.Post("/oauth/github/redirect", h.GitHubRedirect)
	})

	return ServerNew(r, rdb, pool, queries)
}

func TestRedirect(t *testing.T) {
	s := r(t)

	mimiID := s.seed(t, "Mimi Kagari", "")
	mimiToken := s.login(t, mimiID)

	test := []struct {
		Name           string
		Token          string
		StateCheck     bool
		ExpectedStatus int
	}{
		{
			Name:           "Successfully redirect Mimi Kagari",
			Token:          mimiToken,
			ExpectedStatus: http.StatusTemporaryRedirect,
		},
		{
			Name:           "Check state existence",
			Token:          mimiToken,
			StateCheck:     true,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Check redirection route",
			Token:          mimiToken,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Failing due to missing token",
			Token:          "",
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range test {
		t.Run(tt.Name, func(t *testing.T) {
			w := s.do(t, http.MethodPost, "/oauth/github/redirect", tt.Token, "")
			if tt.ExpectedStatus == http.StatusOK {
				location, err := url.Parse(w.Header().Get("Location"))
				if err != nil {
					t.Fatalf("Failed to parse url: %v", err)
				}

				if tt.StateCheck {
					state := location.Query().Get("state")
					_, err = s.redis.Get(context.Background(), "oauth:github:state:"+state).Result()
					if err != nil {
						if err == redis.Nil {
							t.Fatalf("Missing state inside the url: %v", err)
						}
						t.Fatalf("Failed to fetch state: %v", err)
					}
					return
				}

				redirectURI := location.Query().Get("redirect_uri")
				if redirectURI != "http://127.0.0.1:8080/oauth/github/callback" {
					t.Fatalf("Redirect uri doesn't matches the initial one: \nGiven URI: %s \nExpected URI:%s", redirectURI, "http://127.0.0.1:8080/oauth/github/callback")
				}
				return
			}

			if w.Code != tt.ExpectedStatus {
				t.Fatalf("Given status: %d \nExpected status: %d \nResponse: %s", w.Code, tt.ExpectedStatus, w.Body.String())
			}
		})
	}
}
