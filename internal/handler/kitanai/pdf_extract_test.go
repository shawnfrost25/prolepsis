package kitanai_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	filegrpc "prolepsis/internal/file_grpc"
	"prolepsis/internal/handler/kitanai"
	"prolepsis/internal/lib"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestExtractPdf(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("Missing 'DATABASE_URL' inside .env")
	}
	pdfClient := os.Getenv("GRPC_PDF_ADDRESS")
	if pdfClient == "" {
		t.Fatal("Missing 'GRPC_PDF_ADDRESS'")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("Missing 'redisAddr' inside .env")
	}
	redisPSWD := os.Getenv("REDIS_PASSWORD")
	if redisPSWD == "" {
		t.Fatal("Missing 'REDIS_PASSWORD' inside .env")
	}
	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}

	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(timeout, config)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}

	queries := db.New(pool)

	grpcClient, err := filegrpc.NewClient(pdfClient)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPSWD,
		DB:       0,
	})

	h := kitanai.New(nil, queries, nil, grpcClient, redisClient, nil)
	auth.Init(queries, redisClient)

	err = h.Queries.TruncateEverythingBeforeTest(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully delete the users and the sessions, due to error: %v", err)
	}
	err = h.Queries.InsertDummiesInsideUsers(timeout)
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the database due to this error: %v", err)
	}
	err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/create_session.redis")
	if err != nil {
		t.Fatalf("Couldn't successfully insert users inside the database due to this error: %v", err)
	}
	defer func() {
		err = lib.RunMake("../../../", "redis-test", "REDIS_PASS="+redisPSWD, "REDIS_PATH=internal/handler/testdata/clean_session.redis")
		if err != nil {
			t.Fatalf("Couldn't successfully clean mock session token, due to error: %v", err)
		}
	}()

	test := []struct {
		name     string
		method   string
		endpoint string
		mimeType string
		filePath string
		failed   bool
	}{
		{
			name:     "Giving it a healthy pdf",
			method:   "POST",
			endpoint: "/users/pdf/extract",
			mimeType: "application/png",
			// The somersaults it needs to do... scary
			filePath: "../../../rust/src/grpc/test_healthy.pdf",
			failed:   false,
		},
		{
			name:     "Giving it a defected pdf",
			method:   "POST",
			endpoint: "/users/pdf/extract",
			mimeType: "application/pdf",
			filePath: "../../../rust/src/grpc/test_defected.pdf",
			failed:   true,
		},
	}

	r := chi.NewRouter()
	r.Use(auth.Auth_Middleware)
	r.Use(auth.Logger_Middleware)
	r.Post("/users/pdf/extract", h.ExtractPdf)

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			file, err := os.Open(tt.filePath)
			if err != nil {
				t.Fatalf("Expected to open file flawlessly, got: %v", err)
			}
			defer func() {
				_ = file.Close()
			}()

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", filepath.Base(tt.filePath))
			if err != nil {
				t.Fatalf("Failed to create form file: %v", err)
			}
			if _, err = io.Copy(part, file); err != nil {
				t.Fatalf("Failed to copy file contents: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Expected the writer to close, but it didn't: %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, &body)

			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer 00000000000000000000000000000001")

			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.failed {
				if w.Code == 200 {
					t.Errorf("Expected to fail, but gpt status 200")
				}
				return
			}

			if w.Code != 200 {
				t.Errorf("Expected to succeed, but got: \nstatus: %d \nresponse: %v", w.Code, w.Body.String())
			}
		})
	}
}
