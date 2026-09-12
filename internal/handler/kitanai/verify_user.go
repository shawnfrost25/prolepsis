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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

	user, err := h.Queries.FetchPendingRegistrationsInfo(timeout, hashToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn().
				Err(err).
				Int("status", http.StatusNotFound).
				Str("cause", "user_token_not_found").
				Str("input", hashToken).
				Msg("user not found in database")

			lib.Pretty(w, http.StatusNotFound, lib.Error{
				Code:    "NOT_FOUND",
				Message: "The given token doesn't match any user in the database",
				Details: map[string]string{
					"reason": "no rows in the database match the given token",
				},
				TraceID: trace,
			})
			return
		}
		if errors.Is(err, pgconn.ErrConnClosed) {
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

	info, err := h.Queries.SecondStepRegisterUser(timeout, db.SecondStepRegisterUserParams{
		Name:         user.Name,
		DisplayName:  user.Name,
		Sex:          user.Sex,
		BirthDate:    user.BirthDate,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
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

	_ = h.Queries.UpdatePendingStatus(timeout, hashToken)

	err = h.Queries.DeleteWhereDone(timeout)
	if err != nil {
		if errors.Is(err, pgconn.ErrConnClosed) {
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
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_user_deletion").
			Msg("unexpected error while deleting info")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to delete user from pending registrations",
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

	logger.Info().
		Int("status", http.StatusCreated).
		Str("cause", "successfully_created_user").
		Str("id", info.ID.String())

	err = h.Queries.CreateSession(timeout, db.CreateSessionParams{
		UserID: info.ID,
		Token:  hashToken,
	})
	if err != nil {
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

	// Add the thingy in the "Authorization: Bearer" header
	lib.Pretty(w, http.StatusCreated, VerificationResponse{
		Status: "success",
		Token:  token,
	})
}
