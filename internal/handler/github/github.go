package oauthGithub

import (
	"net/http"
	db "prolepsis/internal/db/sqlc"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

type GitHubHandler struct {
	RedisClient *redis.Client
	GH_Config   oauth2.Config
	HTTP_Client http.Client
	Queries     *db.Queries
}

func GH_New(redisClient *redis.Client, ghConfig oauth2.Config, httpClient http.Client, queries *db.Queries) *GitHubHandler {
	return &GitHubHandler{
		RedisClient: redisClient,
		GH_Config:   ghConfig,
		HTTP_Client: httpClient,
		Queries:     queries,
	}
}
