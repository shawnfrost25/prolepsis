package kitanai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"prolepsis/internal/auth"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

type RegisterUser struct {
	Name      *string      `json:"name"`
	Sex       *string      `json:"sex"`
	BirthDate *pgtype.Date `json:"birth_date"`
	Email     *string      `json:"email"`
	Password  *string      `json:"password"`
}

type RegisterResponse struct {
	Name      string      `json:"name"`
	Sex       string      `json:"sex"`
	BirthDate pgtype.Date `json:"age"`
	Email     string      `json:"email"`
	Nonsense  string      `json:"password"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req RegisterUser
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "CreateUser").Logger()
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

	var missingFields []string
	if req.Name == nil {
		missingFields = append(missingFields, "name")
	}
	if req.Sex == nil {
		missingFields = append(missingFields, "sex")
	}
	if req.Email == nil {
		missingFields = append(missingFields, "email")
	}
	if req.BirthDate == nil {
		missingFields = append(missingFields, "birth_date")
	}
	if req.Password == nil {
		missingFields = append(missingFields, "password")
	}
	if len(missingFields) != 0 {
		logger.Warn().
			Int("status", http.StatusBadRequest).
			Str("cause", "missing_required_arguments").
			Interface("missing_fields", missingFields).
			Msg("missing arguments inside the registrations")
		lib.Pretty(w, http.StatusBadRequest, lib.Error{
			Code:    "BAD_REQUEST",
			Message: "The given payload has missing arguments",
			Details: map[string]string{
				"reason": "missing arguments",
				"fix":    "please, don't forget to write all the required information and don't leave nothing blank",
			},
			TraceID: trace,
		})
		return
	}
	timeout, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	emailTimeout, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = h.Queries.DeleteWhereDone(timeout)
	if errors.Is(err, pgconn.ErrConnClosed) {
		logger.Error().
			Err(err).
			Int("status", http.StatusRequestTimeout).
			Str("cause", "timeout").
			Msg("timedout while searching for the user matching the id")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An error occurred while deleting the pending registration",
			Details: map[string]string{
				"reason": "database error",
				"fix":    "Please try again later",
			},
			TraceID: trace,
		})
		return
	}

	domain := "just_a_test.com"
	appName := "Cruddy Inc."
	standardResponse := map[string]string{
		"message": fmt.Sprintf("If the provided information is valid, a verification token has been sent for %s.", appName),
	}
	userExists, err := h.Queries.UserExists(timeout, *req.Email)
	if err != nil {
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
			Str("cause", "database_registration_failed").
			Str("email", *req.Email).
			Msg("unexpected database error during email query")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An unexpected error occurred.",
			TraceID: trace,
		})
		return
	}
	place := lib.ExtractLocation(r)
	device := lib.ExtractDevice(r)
	if userExists {

		subject := fmt.Sprintf("[%s] Security notice: Registration attempt", appName)
		message := fmt.Sprintf(
			"Hello,\n\n"+
				"Someone recently attempted to create an account on %s using your email address.\n\n"+
				"Details of the attempt:\n"+
				"- Time: %s\n"+
				"- Location: %s\n"+
				"- Device: %s\n\n"+
				"If this was you, you can safely ignore this email and sign in to your existing account.\n\n"+
				"If you did not initiate this request, no action is required—your account remains secure.\n\n"+
				"Best regards,\nThe %s Team",
			domain,
			time.Now().Format("January 2, 2006 at 3:04 PM MST"),
			place,
			device,
			appName,
		)

		err = lib.SendLimitedEmail(emailTimeout, logger, h.Queries, h.Mailer.ApiKey, *req.Email, subject, message)
		if err != nil {
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "failed_email_response").
				Str("email", *req.Email).
				Msg("could not deliver email notification, rate limited, email exists")

			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Check the inboxes of the provided email for the given token",
				TraceID: trace,
			})
			return
		}

		logger.Info().
			Int("status", http.StatusOK).
			Str("email", *req.Email).
			Msg("sent duplicate registration notification email")

		lib.Pretty(w, http.StatusOK, standardResponse)
		return
	}

	token, err := auth.CreateToken()
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "token_fetching_failed").
			Str("email", *req.Email).
			Msg("couldn't create token")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unexpected error while creating the token",
			TraceID: trace,
		})
		return
	}

	subject := fmt.Sprintf("Verify your email for %s", appName)

	message := fmt.Sprintf(
		"Hello %s,\n\n"+
			"Thank you for signing up for %s!\n\n"+
			"This verification link will expire in 3 minutes. Please use this token: %v\n\n"+
			"If you did not create an account, you can safely ignore this message.\n\n"+
			"Best regards,\nThe %s Team",
		*req.Name,
		appName,
		token,
		appName,
	)

	exists, err := h.Queries.ExistsInPendingRegistrations(timeout, *req.Email)
	if err != nil {
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
			Str("cause", "database_registration_failed").
			Str("email", *req.Email).
			Msg("unexpected database error during email query")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An unexpected error occurred.",
			TraceID: trace,
		})
		return
	}

	if exists {
		if exists {
			logger.Warn().
				Int("status", http.StatusOK).
				Str("cause", "healthy_token").
				Msg("active pending registration exists, returning standard response")

			lib.Pretty(w, http.StatusOK, standardResponse)
			return
		}
	} else {
		h.Queries.DeleteRegistrationSteps(timeout, *req.Email)
	}

	err = lib.SendEmail(emailTimeout, logger, h.Mailer.ApiKey, *req.Email, subject, message)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_email_response").
			Str("email", *req.Email).
			Msg("could not deliver email notification")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Failed to process request. Please try again later.",
			TraceID: trace,
		})
		return
	}

	hashToken := auth.HashToken(token)
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "failed_password_hashing").
			Str("email", *req.Email).
			Msg("error while hashing the password")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Error while trying to encrypt the given password",
			TraceID: trace,
		})
		return
	}

	err = h.Queries.FirstStepRegisterUser(timeout, db.FirstStepRegisterUserParams{
		Token:        hashToken,
		Name:         *req.Name,
		Sex:          *req.Sex,
		BirthDate:    *req.BirthDate,
		Email:        *req.Email,
		PasswordHash: string(hashPassword),
	})

	if err != nil {
		logger.Error().
			Err(err).
			Int("status", http.StatusRequestTimeout).
			Str("cause", "timeout").
			Msg("timedout while trying to insert user inside pending_registrations")
		lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
			Code:    "REQUEST_TIMEOUT",
			Message: "Timeout while inserting inside database",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("email", *req.Email).
		Msg("successfully created pending user registration")

	lib.Pretty(w, http.StatusOK, standardResponse)
}
