package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	filegrpc "prolepsis/internal/file_grpc"
	"prolepsis/internal/handler/kitanai"
	"prolepsis/internal/lib"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()
	databaseUrl := os.Getenv("DATABASE_URL")
	if databaseUrl == "" {
		panic("missing 'DATABASE_URL' inside .env")
	}
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		panic("missing 'APP_PASSWORD' inside .env")
	}
	grpcAddr := os.Getenv("GRPC_PDF_ADDRESS")
	if grpcAddr == "" {
		panic("missing 'GRPC_PDF_ADDRESS' inside .env")
	}
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lib.Init(zerolog.InfoLevel)

	config, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		panic(err)
	}

	config.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(timeout, config)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	queries := db.New(pool)
	auth.Init(queries)

	values := kitanai.Mailer{
		ApiKey: apiKey,
	}

	// This is on which port Rust will listen
	pdfClient, err := filegrpc.NewClient(grpcAddr)
	if err != nil {
		panic(err)
	}
	// We close the connection when we close the main
	defer pdfClient.Close()

	k := kitanai.Handler{
		Pool:      pool,
		Queries:   queries,
		Mailer:    &values,
		PdfClient: pdfClient,
	}

	// We steal the real IP of the user
	clientIPKey := func(r *http.Request) (string, error) {
		startIP := middleware.GetClientIP(r.Context())
		return httprate.CanonicalizeIP(startIP), nil
	}

	globalLimiter := httprate.LimitBy(150, time.Minute, clientIPKey)

	authLimiter := httprate.LimitBy(10, time.Minute, clientIPKey)
	readLimiter := httprate.LimitBy(60, time.Minute, clientIPKey)
	mutationLimiter := httprate.LimitBy(20, time.Minute, clientIPKey)

	r := chi.NewRouter()
	r.Use(hlog.NewHandler(log.Logger))
	r.Use(globalLimiter)
	r.Use(auth.Logger_Middleware)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(authLimiter)

		r.Post("/users/create", k.CreateUser)
		r.Post("/users/create/verify", k.VerifyRegistration)
		r.Post("/users/login", k.LoginUser)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)

		r.With(readLimiter).Get("/users/{id}", k.GetUserByID)
		r.With(readLimiter).Get("/users", k.GetUserByQuery)
		r.With(mutationLimiter).Patch("/users/update", k.UpdateUserInfo)
		r.With(readLimiter).Post("/users/pdf/extract", k.ExtractPdf)
	})

	fmt.Print("Successfully running on port 8080")
	addr := "0.0.0.0:8080"
	http.ListenAndServe(addr, r)
}
