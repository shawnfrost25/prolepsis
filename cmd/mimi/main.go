package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	filegrpc "prolepsis/internal/file_grpc"
	oauthGithub "prolepsis/internal/handler/github"
	"prolepsis/internal/handler/kitanai"
	kitanaijob "prolepsis/internal/job/kitanai"
	"prolepsis/internal/lib"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
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
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		panic("missing 'REDIS_ADDR' inside .env")
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		panic("missing 'REDIS_PASSWORD' inside .env")
	}
	clientID := os.Getenv("CLIENT_ID")
	if clientID == "" {
		panic("missing 'CLIENT_ID' inside .env")
	}
	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientSecret == "" {
		panic("missing 'CLIENT_SECRET' inside .env")
	}

	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lib.Init(zerolog.DebugLevel)

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

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			panic(err)
		}
	}()

	auth.Init(queries, redisClient)

	values := kitanai.Mailer{
		ApiKey: apiKey,
	}

	// This is on which port Rust will listen
	pdfClient, err := filegrpc.NewClient(grpcAddr)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := pdfClient.Close(); err != nil {
			panic(err)
		}
	}()

	workers := kitanaijob.SetupDeletionWorkers(queries)
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 1,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	k := kitanai.Handler{
		Pool:        pool,
		Queries:     queries,
		Mailer:      &values,
		PdfClient:   pdfClient,
		RedisClient: redisClient,
		RiverClient: riverClient,
	}

	ghConfig := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  "http://127.0.0.1:8080/oauth/github/callback",
		Scopes:       []string{"read:user", "repo"},
	}

	httpClient := http.Client{Timeout: 10 * time.Second}

	g := oauthGithub.GH_New(redisClient, ghConfig, httpClient, queries)

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
		r.Get("/users/create/verify", k.VerifyRegistration)
		r.Post("/users/login", k.LoginUser)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)

		r.With(readLimiter).Get("/users/{id}", k.GetUserByID)
		r.With(readLimiter).Get("/users", k.GetUserByQuery)
		r.With(authLimiter).Delete("/users/delete/target/{id}", k.DeleteUserTarget)
		r.With(authLimiter).Delete("/users/delete/self", k.DeleteUserSelf)
		r.With(mutationLimiter).Patch("/users/update", k.UpdateUserInfo)
		r.With(readLimiter).Post("/users/pdf/extract", k.ExtractPdf)
	})

	r.Group(func(r chi.Router) {
		r.Route("/oauth", func(r chi.Router) {
			r.Use(auth.Auth_Middleware)

			r.With(authLimiter).Post("/github/redirect", g.GitHubRedirect)
			r.With(authLimiter).Get("/github/callback", g.GitHubCallback)
			r.With(authLimiter).Post("/github/callback/mock", g.GitHubCallbackMock)
		})
	})

	fmt.Print("Successfully running on port 8080")
	addr := "0.0.0.0:8080"
	if err := http.ListenAndServe(addr, r); err != nil {
		panic(err)
	}
}
