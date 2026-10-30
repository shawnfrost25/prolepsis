package kitanai

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
	red     *redis.Client
	queries *db.Queries
	pool    *pgxpool.Pool
}

func ServerNew(router http.Handler, redis *redis.Client, queries *db.Queries, pool *pgxpool.Pool) *Server {
	return &Server{
		router:  router,
		red:     redis,
		queries: queries,
		pool:    pool,
	}
}

// 'do' executes an HTTP request against the server router, abstracting away JSON marshaling, request creation, and header setup.
// It returns *httptest.ResponseRecorder so the calling test can assert on the response status code and body.
func (s *Server) do(t *testing.T, method string, endpoint string, authHeader string, body any) *httptest.ResponseRecorder {
	// 't.Helper()' marks this function as a test helper so Go's test runner attributes any failures to the calling test line rather than inside this helper.
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body error: %v", err)
		}
		rd = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, endpoint, rd)
	w := httptest.NewRecorder()

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	s.router.ServeHTTP(w, req)
	return w
}

// 'seed' inserts a mock user into the database, dynamically deriving the email address from the provided name to satisfy the unique constraint across test cases.
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
	_, err := s.red.Set(context.Background(), "session:token:"+tokenHash, userID.String(), 60*24*time.Hour).Result()
	if err != nil {
		t.Fatalf("Failed to insert session token inside redis: %v", err)
	}

	return "Bearer " + token
}
