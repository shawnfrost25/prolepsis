package kitanai

import (
	"context"
	"fmt"
	"net/http"
	"prolepsis/internal/auth"
	"prolepsis/internal/errlog"
	"prolepsis/internal/failure"
	"prolepsis/internal/lib"
	"time"

	"github.com/bytedance/sonic"
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
	logger := zerolog.Ctx(r.Context()).With().Str("op", "create_user").Logger()
	trace, ok := auth.GetTrace(r.Context())
	if !ok {
		err := failure.EmptyValue("trace_context")
		err = failure.Wrap("kitanai.http.trace.context.fetch", err)
		errlog.Http(logger, w, "trace_context", failure.InternalEmptyValueError, err, trace)
		return
	}

	err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = failure.Wrap("kitanai.json.create.payload.decode", err)
		errlog.Json(logger, w, "user_create", trace, err)
		return
	}

	var missingFields []string
	if req.Name == nil || *req.Name == "" {
		missingFields = append(missingFields, "name")
	}
	if req.Sex == nil || *req.Sex == "" {
		missingFields = append(missingFields, "sex")
	}
	if req.Email == nil || *req.Email == "" {
		missingFields = append(missingFields, "email")
	}
	if req.BirthDate == nil {
		missingFields = append(missingFields, "birth_date")
	}
	if req.Password == nil || *req.Password == "" {
		missingFields = append(missingFields, "password")
	}
	if len(missingFields) != 0 {
		err := failure.EmptyValue("registration_fields")
		err = failure.Wrap("kitanai.http.create.payload.existence_check", err)
		logger = logger.With().Interface("missing_fields", missingFields).Logger()
		errlog.Http(logger, w, "registration_fields", failure.EmptyValueError, err, trace)
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
		err = failure.Wrap("kitanai.database.user.existence_check", err)
		errlog.Database(logger, w, "user_existence_check", trace, err)
		return
	}
	place := lib.ExtractLocation(r)
	device := lib.ExtractDevice(r)
	token, err := auth.CreateToken()
	if err != nil {
		err = failure.Wrap("kitanai.http.registration.token.create", err)
		errlog.Http(logger, w, "registration_token", failure.GenerationError, err, trace)
		return
	}

	if userExists {
		// "doesn_exist = true" means that SetNX inserted the entries to Redis (meaning there was no rate limit)
		doesnt_exists, err := h.RedisClient.SetNX(timeout, "pending:registration:email:"+*req.Email, 1, 24*time.Hour).Result()
		// If an error happened while trying to insert the rate limited email
		if err != nil {
			err = failure.Wrap("kitanai.redis.registration.email.insert", err)
			errlog.Redis(logger, w, "registration_email_insert", err, trace)
			return
		}

		// If the email rate limit doesn't exists (it returns true if it inserted stuff), we send a warning email (the rate limit get created)
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

			err = lib.SendEmail(emailTimeout, logger, h.Mailer.Email, h.Mailer.AppPassword, *req.Email, subject, message)
			if err != nil {
				err = failure.Wrap("kitanai.http.email.send", err)
				errlog.General(logger, w, "email_send", failure.InternalGeneralError, err, trace)
				return
			}
			logger.Info().
				Int("status", http.StatusOK).
				Str("email", *req.Email).
				Msg("successfully dispatched duplicate account registration security notice")
			lib.Pretty(w, http.StatusOK, standardResponse)
			return
		}
		err = failure.RateLimit()
		err = failure.Wrap("kitanai.http.registration.email.rate_limit_check", err)
		logger = logger.With().Str("email", *req.Email).Logger()
		errlog.Http(logger, w, "registration_email", failure.RateLimitError, err, trace)
		return
	}

	// Since the "if" above didn't trigger, it means that there is no user in the database, so we can safely create one
	subject := fmt.Sprintf("Verify your email for %s", appName)
	message := fmt.Sprintf(
		"Hello %s,\n\n"+
			"Thank you for signing up for %s!\n\n"+
			"This verification link will expire in 5 minutes. Please click on the provided link: http://127.0.0.1:8080/users/create/verify?token=%v\n\n"+
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
		err = failure.Wrap("kitanai.http.password.hash", err)
		errlog.Http(logger, w, "password_hash", failure.GenerationError, err, trace)
		return
	}
	err = lib.SendEmail(emailTimeout, logger, h.Mailer.Email, h.Mailer.AppPassword, *req.Email, subject, message)
	if err != nil {
		err = failure.Wrap("kitanai.http.email.send", err)
		errlog.General(logger, w, "email_send", failure.InternalGeneralError, err, trace)
		return
	}

	insertedFields, err := h.RedisClient.HSetEXWithArgs(timeout, "pending:registration:token:"+hashToken, &redis.HSetEXOptions{
		Condition:      "FNX",
		ExpirationType: redis.HSetEXExpirationEX,
		ExpirationVal:  300,
	},
		"name", *req.Name,
		"sex", *req.Sex,
		// time.DateOnly -> YYYY-MM-DD
		"birth_date", req.BirthDate.Time.Format(time.DateOnly),
		"email", *req.Email,
		"password_hash", string(hashPassword),
	).Result()
	if err != nil {
		err = failure.Wrap("kitanai.redis.registration.token.insert", err)
		errlog.Redis(logger, w, "registration_token_insert", err, trace)
		return
	}
	// Using "FNX" (Field Not Exists) ensures fields are only inserted if they do not already exist. A return value of 0 means the operation skipped because the registration record already exists. Otherwise, it returns the total number of newly created hash fields.
	if insertedFields == 0 {
		err = failure.Conflict("registration_token")
		err = failure.Wrap("kitanai.redis.registration.token.existence_check", err)
		errlog.Http(logger, w, "registration_token", failure.ConflictError, err, trace)
		return
	}

	logger.Info().
		Int("status", http.StatusOK).
		Str("email", *req.Email).
		Msg("created the pending registration")
	lib.Pretty(w, http.StatusOK, standardResponse)
}
