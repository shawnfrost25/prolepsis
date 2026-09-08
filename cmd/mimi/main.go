package main

import (
	"context"
	"net/http"
	"os"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/handler/kitanai"
	"prolepsis/internal/lib"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	timeout, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

	values := kitanai.Mailer{
		ApiKey: apiKey,
	}

	k := kitanai.Handler{
		Pool:    pool,
		Queries: queries,
		Mailer:  &values,
	}

	r := chi.NewRouter()
	r.Use(hlog.NewHandler(log.Logger))
	r.Use(auth.Logger_Middleware)
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Post("/users/create", k.CreateUser)
		r.Post("/users/create/verify", k.VerifyRegistration)
		r.Post("/users/login", k.LoginUser)
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.Auth_Middleware)
		r.Get("/users/{id}", k.GetUserByID)
		r.Get("/users", k.GetUserByQuery)
		r.Patch("/users/update", k.UpdateUserInfo)
	})

	addr := "0.0.0.0:8080"

	http.ListenAndServe(addr, r)
}
