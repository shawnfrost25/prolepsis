package oauthGithub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	router  http.Handler
	redis   *redis.Client
	pool    *pgxpool.Pool
	queries *db.Queries
}

func ServerNew(handler http.Handler, rdb *redis.Client, pool *pgxpool.Pool, queries *db.Queries) *Server {
	return &Server{
		router:  handler,
		redis:   rdb,
		pool:    pool,
		queries: queries,
	}
}

func (s *Server) do(t *testing.T, method string, endpoint string, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to turn body into bytes: %v", err)
		}

		b = bytes.NewReader(bodyBytes)
	}
	r := httptest.NewRequest(method, endpoint, b)

	if b != nil {
		r.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		r.Header.Set("Authorization", token)
	}

	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, r)
	return w
}

func (s *Server) seed(t *testing.T, name string, role string) pgtype.UUID {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Sheena"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to encrypt password: %v", err)
	}

	if role == "" {
		role = "user"
	}
	cleanName := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	id, err := s.queries.InsertMockUser(context.Background(), db.InsertMockUserParams{
		Name:         name,
		DisplayName:  name,
		Role:         db.UserRole(role),
		Sex:          "prefer_not_to_specify",
		BirthDate:    pgtype.Date{Time: time.Date(1999, 12, 06, 1, 1, 1, 1, time.Local), Valid: true},
		Email:        cleanName + "@gmail.com",
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		t.Fatalf("Failed to insert user in database error: %v", err)
	}

	return id
}

// We use "login" to insert the session token inside redis and return it, so we can use it
func (s *Server) login(t *testing.T, userID pgtype.UUID) string {
	t.Helper()
	m := make([]byte, 32)
	_, _ = rand.Read(m)
	token := hex.EncodeToString(m)
	tokenHash := auth.HashToken(token)
	_, err := s.redis.Set(context.Background(), "session:token:"+tokenHash, userID.String(), 60*24*time.Hour).Result()
	if err != nil {
		t.Fatalf("Failed to insert session token inside redis: %v", err)
	}

	return "Bearer " + token
}
