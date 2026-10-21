package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
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
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("kitanai.http.trace.context.fetch", err)
		errlog.Http(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}

	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = failure.Wrap("kitanai.json.login.payload.decode", err)
		errlog.Json(logger, w, "user_login", trace, err)
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
		err = failure.EmptyValue(missing)
		err = failure.Wrap("kitanai.http.login.payload.existence_check", err)
		errlog.Http(logger, w, missing, failure.EmptyValueError, err, trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	info, err := h.Queries.EmailForInfo(timeout, req.Email)
	if err != nil {
		err = failure.Wrap("kitanai.database.user.fetch_via_email", err)
		errlog.Database(logger, w, "user_lookup", trace, err)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(info.PasswordHash), []byte(req.Password))
	if err != nil {
		err = failure.Wrap("kitanai.http.password.compare", err)
		errlog.Http(logger, w, "user_login", failure.UnauthorizedError, err, trace)
		return
	}

	token, err := auth.CreateToken()
	if err != nil {
		err = failure.Wrap("kitanai.http.session.token.create", err)
		errlog.Http(logger, w, "session_token", failure.GenerationError, err, trace)
		return
	}

	hashToken := auth.HashToken(token)

	_, err = h.RedisClient.Set(timeout, "session:token:"+hashToken, info.ID.String(), 5184000*time.Second).Result()
	if err != nil {
		err = failure.Wrap("kitanai.redis.session.token.insert", err)
		errlog.Redis(logger, w, "session_token_insert", err, trace)
		return
	}

	insertedFields, err := h.RedisClient.SAdd(timeout, "session:id:"+info.ID.String(), hashToken).Result()
	if err != nil {
		err = failure.Wrap("kitanai.redis.session.id.insert", err)
		errlog.Redis(logger, w, "session_id_insert", err, trace)
		return
	}
	if insertedFields == 0 {
		err = failure.Conflict("session_id")
		err = failure.Wrap("kitanai.redis.session.id.existence_check", err)
		errlog.Http(logger, w, "session_id", failure.ConflictError, err, trace)
		return
	}
	_, err = h.RedisClient.Expire(timeout, "session:id:"+info.ID.String(), 5184000*time.Second).Result()
	if err != nil {
		err = failure.Wrap("kitanai.redis.session.id.expire", err)
		errlog.Redis(logger, w, "session_id_expire", err, trace)
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
