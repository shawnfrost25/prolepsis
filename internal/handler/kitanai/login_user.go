package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	logger := zerolog.Ctx(r.Context()).With().Str("op", "login_user").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "trace", w, trace)
		return
	}

	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errlog.DecodeError(logger, w, trace, err)
		return
	}

	if req.Email == "" || req.Password == "" {
		missing := "required_arguments"
		if req.Email == "" && req.Password != "" {
			missing = "email_argument"
		}
		if req.Email != "" && req.Password == "" {
			missing = "password_argument"
		}
		errlog.EmptyValueError(logger, missing, w, trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var pgErr *pgconn.PgError

	info, err := h.Queries.EmailForInfo(timeout, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			errlog.NoRowsError(logger, "user_lookup_by_email", w, trace)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_lookup_by_email", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "user_lookup_by_email_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "user_lookup_by_email", w, trace, err)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(info.PasswordHash), []byte(req.Password))
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusUnauthorized).
			Str("code", "invalid_password").
			Msg("incorrect password")
		lib.Pretty(w, http.StatusUnauthorized, lib.Error{
			Code:    "UNAUTHORIZED",
			Message: "Invalid email or password",
			TraceID: trace,
		})
		return
	}

	token, err := auth.CreateToken()
	if err != nil {
		errlog.GenerateError(logger, "token_create", w, trace, err)
		return
	}

	hashToken := auth.HashToken(token)

	_, err = h.RedisClient.Set(timeout, "session:token:"+hashToken, info.ID.String(), 5184000*time.Second).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "session_token_insert", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "session_token_insert", w, trace, err)
		return
	}

	insertedFields, err := h.RedisClient.SAdd(timeout, "session:id:"+info.ID.String(), hashToken).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "session_id_insert", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "session_id_insert", w, trace, err)
		return
	}
	if insertedFields == 0 {
		errlog.ConflictError(logger, "session_id", w, trace)
		return
	}
	_, err = h.RedisClient.Expire(timeout, "session:id:"+info.ID.String(), 5184000*time.Second).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "session_id_ttl", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "session_id_ttl", w, trace, err)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Msg("successfully created new session")

	lib.Pretty(w, http.StatusOK, LoginResponse{
		Status: "success",
		Token:  token,
	})
}
