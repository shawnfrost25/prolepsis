package kitanai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
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
	timeout, cancelDB := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancelDB()

	emailTimeout, cancelEmail := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancelEmail()

	domain := "just_a_test.com"
	appName := "Cruddy Inc."
	standardResponse := map[string]string{
		"message": fmt.Sprintf("If the provided information is valid, a verification token has been sent for %s via email.", appName),
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
			Msg("unexpected database error while checking if an user matching the email exists or not")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "An unexpected error occurred.",
			TraceID: trace,
		})
		return
	}
	place := lib.ExtractLocation(r)
	device := lib.ExtractDevice(r)
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

	if userExists {
		// doesn_exist = true (if the rate limit get inserted); else it will be false, meaning the rate limit is already in between the entries
		doesnt_exists, err := h.RedisClient.SetNX(timeout, "pending:registration:email:"+*req.Email, 1, 24*time.Hour).Result()
		// If an error happened wile trying to insert the rate limited email
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error().
					Err(err).
					Int("status", http.StatusRequestTimeout).
					Str("cause", "timeout").
					Msg("timeout while trying to verify or insert rate limit constraints")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIMEOUT",
					Message: "Timeout while verifying system availability. Please try again.",
					TraceID: trace,
				})
				return
			}
			logger.Error().
				Err(err).
				Int("status", http.StatusInternalServerError).
				Str("cause", "unrecognized").
				Msg("unrecognized error while validating rate limit constraints")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized structural fault while verifying request constraints",
				TraceID: trace,
			})
			return
		}

		// If the email rate limit doesn't exists, we send a warning email (the rate limit get created)
		if doesnt_exists {
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

			err = lib.SendEmail(emailTimeout, logger, h.Mailer.ApiKey, *req.Email, subject, message)
			if err != nil {
				logger.Error().
					Err(err).
					Int("status", http.StatusInternalServerError).
					Str("cause", "failed_email_response").
					Str("email", *req.Email).
					Msg("could not deliver security notification email to existing user account")
				_ = h.RedisClient.Del(timeout, "pending:registration:email:"+*req.Email).Err()
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "EMAIL_DELIVERY_FAILED",
					Message: "Failed to securely dispatch notification mail. Please try again shortly.",
					TraceID: trace,
				})
				return
			}

			logger.Info().
				Int("status", http.StatusOK).
				Str("email", *req.Email).
				Msg("successfully dispatched duplicate account registration security notice")

			lib.Pretty(w, http.StatusOK, standardResponse)
			return
		}
		logger.Warn().
			Str("email", *req.Email).
			Int("status", http.StatusTooManyRequests).
			Str("cause", "rate_limit").
			Msg("registration notice request rejected because an active rate limit key exists for this email")

		lib.Pretty(w, http.StatusTooManyRequests, lib.Error{
			Code:    "TOO_MANY_REGISTRATION_REQUESTS",
			Message: "An account security email was already sent recently. Please check your inbox or wait before trying again.",
			TraceID: trace,
		})
		return
	}

	// Since the "if" above didn't trigger, it means that there is no user in the database, so we can safely create one
	subject := fmt.Sprintf("Verify your email for %s", appName)
	message := fmt.Sprintf(
		"Hello %s,\n\n"+
			"Thank you for signing up for %s!\n\n"+
			"This verification link will expire in 3 minutes. Please click on the provided link: http://127.0.0.1:8080/users/create/verify/%v\n\n"+
			"If you did not create an account, you can safely ignore this message.\n\n"+
			"Best regards,\nThe %s Team",
		*req.Name,
		appName,
		token,
		appName,
	)

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

	insertedFields, err := h.RedisClient.HSetEXWithArgs(timeout, "pending:registration:token:"+hashToken, &redis.HSetEXOptions{
		Condition:      "FNX",
		ExpirationType: redis.HSetEXExpirationEX,
		ExpirationVal:  180,
	},
		"name", *req.Name,
		"sex", *req.Sex,
		// time.DateOnly -> YYYY-MM-DD
		"birth_date", req.BirthDate.Time.Format(time.DateOnly),
		"email", *req.Email,
		"password_hash", string(hashPassword),
	).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Error().
				Err(err).
				Int("status", http.StatusRequestTimeout).
				Str("cause", "timeout").
				Msg("timeout while trying to insert redis Hash inside the entries")
			lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
				Code:    "REQUEST_TIMEOUT",
				Message: "Timeout while trying to insert data through redis entries",
				TraceID: trace,
			})
			return
		}
		logger.Error().
			Err(err).
			Int("status", http.StatusInternalServerError).
			Str("cause", "unrecognized").
			Msg("unrecognized error while trying to insert data through redis entries")
		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "Unrecognized error while trying to insert redis data",
			TraceID: trace,
		})
		return
	}
	// Using "FNX" (Field Not Exists) ensures fields are only inserted if they do not already exist. A return value of 0 means the operation skipped because the registration record already exists. Otherwise, it returns the total number of newly created hash fields.
	if insertedFields == 0 {
		logger.Warn().
			Int("status", http.StatusConflict).
			Str("cause", "pending_registration_already_exists").
			Msg("this pending registration already exists as entry")
		lib.Pretty(w, http.StatusConflict, lib.Error{
			Code:    "CONFLICT",
			Message: "You already have a working session, please check the email and click the link on it",
			Details: map[string]string{
				"reason": "key already exists inside the pending registrations",
				"fix":    "don't try registrating, simply enter the email and click the link send by us",
			},
			TraceID: trace,
		})
		return
	}

	if insertedFields != 5 {
		logger.Error().
			Int64("inserted", insertedFields).
			Int("status", http.StatusInternalServerError).
			Msg("unexpected number of hash fields inserted into Redis")

		lib.Pretty(w, http.StatusInternalServerError, lib.Error{
			Code:    "PARTIAL_WRITE_ERROR",
			Message: "Failed to write complete registration details into cache",
			TraceID: trace,
		})
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("email", *req.Email).
		Msg("successfully created the pending registration")

	lib.Pretty(w, http.StatusOK, standardResponse)
}
