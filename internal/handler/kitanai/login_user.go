package kitanai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "LoginUser").Logger()
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
				"fix":    "follow the recommendations and given format to successfully continue",
			},
			TraceID: trace,
		})
		return
	}

	if req.Email == "" || req.Password == "" {
		missing := "missing_required_arguments"
		if req.Email == "" && req.Password != "" {
			missing = "missing_email_argument"
		}
		if req.Email != "" && req.Password == "" {
			missing = "missing_password_argument"
		}
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", missing).
			Msg("missing login arguments")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given request contains missing fields!",
			Details: map[string]string{
				"reason": "missing arguments",
				"fix":    "please, provide the arguments for both email and password",
			},
			TraceID: trace,
		})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_password_hashing").
			Str("email", req.Email).
			Msg("error while hashing the password")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while trying to encrypt the given password",
			TraceID: trace,
		})
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()

	info, err := h.queries.EmailForInfo(timeout, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_email_not_found").
				Str("email", req.Email).
				Msg("user not found in database")

			lib.Pretty(w, http.StatusUnauthorized, lib.Error{
				Code:    "UNAUTHORIZED",
				Message: "Invalid email or password",
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timedout while searching for the user matching the email")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while searching for matching email",
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
			Str("cause", "database_email_query_failed").
			Str("email", req.Email).
			Msg("unexpected error while fetching user information")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unexpected error happened while matching the given email",
			TraceID: trace,
		})
		return
	}

	err = bcrypt.CompareHashAndPassword(passwordHash, []byte(req.Password))
	if err != nil {
		logger.Warn().
			Err(err).
			Int("status", http.StatusUnauthorized).
			Str("cause", "invalid_password_for_login").
			Str("input", req.Password).
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
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "token_fetching_failed").
			Str("email", req.Email).
			Msg("couldn't create token")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unexpected error while creating the token",
			TraceID: trace,
		})
		return
	}

	hashBytes := sha256.Sum256([]byte(token))
	hashToken := hex.EncodeToString(hashBytes[:])

	h.queries.CreateSession(timeout, db.CreateSessionParams{
		UserID: info.ID,
		Token:  hashToken,
	})

	logger.Info().
		Int("status", http.StatusOK).
		Str("email", req.Email).
		Msg("successfully created session")
	lib.Pretty(w, http.StatusOK, LoginResponse{
		Status: "success",
		Token:  token,
	})

}
