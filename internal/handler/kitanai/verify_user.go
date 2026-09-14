package kitanai

import (
	"context"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

type VerificationRequest struct {
	Token string `json:"token"`
}

type VerificationResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

func (h *Handler) VerifyRegistration(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "VerifyRegistrations").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		logger.Error().
			Int("status", http.StatusInternalServerError).
			Str("cause", "missing_tracing_context").
			Msg("handler invoked without trace ID in context")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An internal server error occurred",
			Details: map[string]string{
				"reason": "request context pipeline uninitialized",
			},
		})
		return
	}

	var req VerificationRequest
	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_decode_request").
			Msg("couldn't decode request")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given request is invalid. Failed to decode the request.",
			Details: map[string]string{
				"reason": "invalid request",
				"fix":    "enter the email you added in the registrations and check for the token, copy it and paste here",
			},
			TraceID: trace,
		})
		return
	}

	if req.Token == "" {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_arguments").
			Msg("missing token in request")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "There is no token in the request, please retry",
			Details: map[string]string{
				"reason": "missing token",
				"fix":    "enter the email you added in the registrations and check for the token, copy it and paste here",
			},
			TraceID: trace,
		})
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	hashToken := auth.HashToken(req.Token)

	if len(hashToken) != 64 {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "invalid_token_request").
			Str("token", req.Token).
			Msg("the given token is malformed")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given token is malformed, please retry",
			Details: map[string]string{
				"reason": "malformed token",
				"fix":    "enter the email you added in the registrations and check for the token, copy it and paste here",
			},
			TraceID: trace,
		})
		return
	}

	val, err := h.RedisClient.HGetAll(timeout, "pending:registration:token:"+hashToken).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Str("input", hashToken).
				Msg("timedout while searching for the user matching the token")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while validating the token",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_pending_registrations_fetching").
			Str("input", hashToken).
			Msg("unexpected error while fetching info")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Database fetching failed",
			TraceID: trace,
		})
		return
	}
	if len(val) == 0 {
		logger.Warn().
			Err(err).
			Int("status", http.StatusNotFound).
			Str("cause", "user_token_not_found").
			Str("input", hashToken).
			Msg("user not found in database, token expired")

		lib.Pretty(w, http.StatusNotFound, lib.Error{
			Code:    "NOT_FOUND",
			Message: "The given token doesn't match any user in the database, token expired",
			Details: map[string]string{
				"reason": "no rows in the database match the given token",
				"fix":    "retry the registration, so it will send another token",
			},
			TraceID: trace,
		})
		return
	}

	var birth pgtype.Date
	birth1, err := time.Parse(time.DateOnly, val["birth_date"])
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "birth_date_error").
			Msg("couldn't safely turn birt_date (type string) to birth_date (type pgtype.Date)")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while parsing birth_date",
			TraceID: trace,
		})
		return
	}

	birth = pgtype.Date{Time: birth1, Valid: true}

	id, err := uuid.NewRandom()
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "id_generation_error").
			Msg("couldn't safely create an uuid")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while creating uuid",
			TraceID: trace,
		})
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
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Str("input", hashToken).
				Msg("timedout while inserting data to database")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while inserting data",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_user_creation").
			Msg("unexpected error while creating user")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to create user from pending registrations to users",
			TraceID: trace,
		})
		return
	}

	token, err := auth.CreateToken()
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "token_fetching_failed").
			Msg("couldn't create token")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unexpected error while creating the token",
			TraceID: trace,
		})
		return
	}
	hashToken = auth.HashToken(token)

	_, err = h.RedisClient.Set(timeout, "session:token:"+hashToken, info.ID, 5184000*time.Second).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Str("input", hashToken).
				Msg("timedout while trying to create a new session")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while trying to create a new session",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "token_creation_failed").
			Str("id", info.ID.String()).
			Msg("couldn't create token inside database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while trying to create the session token",
		})
		return
	}
	inserted_fields, err := h.RedisClient.SAdd(timeout, "session:id:"+info.ID.String(), hashToken).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Str("input", hashToken).
				Msg("timedout while trying to create a new session")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while trying to create a new session",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "token_creation_failed").
			Str("id", info.ID.String()).
			Msg("couldn't create token inside database")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while trying to create the session token",
		})
		return
	}
	if inserted_fields == 0 {
		logger.Error().
			Str("user_id", info.ID.String()).
			Msg("SAdd returned 0 during registration. Token collision or duplicate request detected.")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "REGISTRATION_SESSION_ERROR",
			Message: "An error occurred while establishing your session. Please try logging in.",
			TraceID: trace,
		})
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
		Token:  token,
	})
}
