package webhook

import (
	db "prolepsis/internal/db/sqlc"

	"github.com/redis/go-redis/v9"
)

type WebhookHandler struct {
	Queries     *db.Queries
	RedisClient *redis.Client
}

func New(queries *db.Queries, redisClient *redis.Client) *WebhookHandler {
	return &WebhookHandler{
		Queries:     queries,
		RedisClient: redisClient,
	}
}
