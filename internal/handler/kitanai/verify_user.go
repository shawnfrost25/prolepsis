package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type VerificationResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

func (h *Handler) VerifyRegistration(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "verify_registration").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "trace", w, trace)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		errlog.EmptyValueError(logger, "token_query", w, trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	hashToken := auth.HashToken(token)

	if len(hashToken) != 64 {
		errlog.MalformedError(logger, "token_length", w, trace)
		return
	}

	val, err := h.RedisClient.HGetAll(timeout, "pending:registration:token:"+hashToken).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "registration_token_fetch", w, trace, err)
			return
		}
		if redis.IsAuthError(err) {
			errlog.RedisAuthenticationError(logger, w, trace, err)
			return
		}
		errlog.UnexpectedError(logger, "registration_token_fetch", w, trace, err)
		return
	}
	if len(val) == 0 {
		errlog.RedisNilError(logger, "registration_token", w, trace, err)
		return
	}

	var birth pgtype.Date
	birth1, err := time.Parse(time.DateOnly, val["birth_date"])
	if err != nil {
		errlog.ParsingError(logger, "birth_date_format", w, trace, err)
		return
	}

	birth = pgtype.Date{Time: birth1, Valid: true}

	id, err := uuid.NewRandom()
	if err != nil {
		errlog.GenerateError(logger, "uuid", w, trace, err)
		return
	}
	uuid := pgtype.UUID{Bytes: id, Valid: true}

	var pgErr *pgconn.PgError

	info, err := h.Queries.SecondStepRegisterUser(timeout, db.SecondStepRegisterUserParams{
		ID:           uuid,
		Name:         val["name"],
		DisplayName:  val["name"],
		Sex:          val["sex"],
		BirthDate:    birth,
		Email:        val["email"],
		PasswordHash: val["password_hash"],
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			errlog.DeadlineExceededError(logger, "user_create", w, trace, err)
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			errlog.ErrConnClosed(logger, w, trace, err)
			return
		}
		if errors.As(err, &pgErr) {
			errlog.PgConnError(logger, w, trace, "user_create_failed", pgErr)
			return
		}
		errlog.UnexpectedError(logger, "user_create", w, trace, err)
		return
	}

	tokenS, err := auth.CreateToken()
	if err != nil {
		errlog.GenerateError(logger, "token", w, trace, err)
		return
	}
	hashToken = auth.HashToken(tokenS)

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
		Int("status", http.StatusCreated).
		Str("cause", "successfully_created_user_session").
		Str("id", info.ID.String()).
		Msg("successfully managed to create both user and session")

	// Add the thingy in the "Authorization: Bearer" header
	lib.Pretty(w, http.StatusCreated, VerificationResponse{
		Status: "success",
		Token:  tokenS,
	})
}
