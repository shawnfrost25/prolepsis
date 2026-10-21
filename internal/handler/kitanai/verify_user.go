package kitanai

import (
	"context"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("kitanai.http.trace.context.fetch", err)
		errlog.Http(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		err := failure.EmptyValue("token_query")
		err = failure.Wrap("kitanai.http.query.token.validate", err)
		errlog.Http(logger, w, "token_query", failure.EmptyValueError, err, trace)
		return
	}

	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	hashToken := auth.HashToken(token)

	if len(hashToken) != 64 {
		err := failure.Invalid("token_length", "hashed token length is not 64 characters")
		err = failure.Wrap("kitanai.http.query.token.length_check", err)
		errlog.Http(logger, w, "token_length", failure.InvalidError, err, trace)
		return
	}

	val, err := h.RedisClient.HGetAll(timeout, "pending:registration:token:"+hashToken).Result()
	if err != nil {
		err = failure.Wrap("kitanai.redis.registration.token.fetch", err)
		errlog.Redis(logger, w, "registration_token_fetch", err, trace)
		return
	}
	if len(val) == 0 {
		err = failure.NotFound("registration_token")
		err = failure.Wrap("kitanai.redis.registration.token.existence_check", err)
		errlog.Http(logger, w, "registration_token", failure.NotFoundError, err, trace)
		return
	}

	var birth pgtype.Date
	birth1, err := time.Parse(time.DateOnly, val["birth_date"])
	if err != nil {
		err = failure.Wrap("kitanai.http.registration.birth_date.parse", err)
		errlog.Http(logger, w, "birth_date", failure.MalformedError, err, trace)
		return
	}

	birth = pgtype.Date{Time: birth1, Valid: true}

	id, err := uuid.NewRandom()
	if err != nil {
		err = failure.Wrap("kitanai.http.user.uuid.generate", err)
		errlog.Http(logger, w, "user_uuid", failure.GenerationError, err, trace)
		return
	}
	uuid := pgtype.UUID{Bytes: id, Valid: true}

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
		err = failure.Wrap("kitanai.database.user.create", err)
		errlog.Database(logger, w, "user_create", trace, err)
		return
	}

	tokenS, err := auth.CreateToken()
	if err != nil {
		err = failure.Wrap("kitanai.http.session.token.create", err)
		errlog.Http(logger, w, "session_token", failure.GenerationError, err, trace)
		return
	}
	hashToken = auth.HashToken(tokenS)

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
