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
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
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

	client, err := filegrpc.NewClient(pdfClient)
	if err != nil {
		t.Fatalf("Expected to run flawlessly, got: %v", err)
	}

	h := kitanai.New(pool, queries, nil, client)
	auth.Init(queries)

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
			if err := file.Close(); err != nil {
				t.Fatalf("Expected the file to close, but it didn't: %v", err)
			}

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, _ := writer.CreateFormFile("file", filepath.Base(tt.filePath))
			_, _ = io.Copy(part, file)
			if err := writer.Close(); err != nil {
				t.Fatalf("Expected the writer to close, but it didn't: %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.endpoint, &body)

			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer 711a89dd67a2fe9e0e8a0972dd3847e29c0bda32bc2cd8b671b4a277dda9c896")

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
