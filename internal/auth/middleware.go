package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/lib"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type contextKey string

const traceCtx contextKey = "userTrace"
const idCtx contextKey = "userID"
const roleCtx contextKey = "userRole"

var queries *db.Queries

// Workflow without this function:
// We delcare 'queries' (a placeholder) -> function starts for real -> no value given to 'queries' -> defaults to 'nil' (because it has a pointer, it's '*db.Queries')

// Workflow with this function:
// We delcare 'queries' (a placeholder) -> we use this function to give it a value -> it has a value, no more a placeholder, finishig as we expect it to
func Init(q *db.Queries) {
	queries = q
}

func Auth_Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			lib.Pretty(w, http.StatusUnauthorized, lib.Error{
				Code:    "UNAUTHORIZED",
				Message: "Cannot proceed without a token",
				Details: map[string]string{
					"reason": "missing token",
					"fix":    "Include the 'Authorization: Bearer <token>' header in your request",
				},
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)

		if len(parts) != 2 || parts[0] != "Bearer" {
			lib.Pretty(w, http.StatusUnauthorized, lib.Error{
				Code:    "UNAUTHORIZED",
				Message: "The given token is malformed",
				Details: map[string]string{
					"reason": "malformed token",
					"fix":    "Ensure the header format strictly follows 'Bearer <token>'",
				},
			})
			return
		}

		hashToken := HashToken(parts[1])

		timeout, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
		defer cancel()

		user, err := queries.CheckToken(timeout, hashToken)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				lib.Pretty(w, http.StatusUnauthorized, lib.Error{
					Code:    "UNAUTHORIZED",
					Message: "The provided token is invalid or expired",
					Details: map[string]string{
						"reason": "token not found or expired",
						"fix":    "Re-authenticate to obtain a new token",
					},
				})
				return
			}

			log.Error().Err(err).Msg("CheckToken failed")
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

		_ = queries.UpdateSession(timeout, hashToken)

		ctx := context.WithValue(r.Context(), idCtx, user.ID)
		ctx = context.WithValue(ctx, roleCtx, user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logger_Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe value insertion by checking if everything is "ok" first
		var userID string
		valueID, ok := r.Context().Value(idCtx).(pgtype.UUID)
		if ok && valueID.Valid {
			userID = valueID.String()
		}
		var userRole string
		valueRole, ok := r.Context().Value(roleCtx).(db.UserRole)
		if ok {
			userRole = string(valueRole)
		}

		// Trying to strip the port from the IP - else defaulting to the IP with port (why people added it?!?!?)
		userIP, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			userIP = r.RemoteAddr
		}

		// Creating the trace
		bytes := make([]byte, 8)
		_, _ = rand.Read(bytes)
		trace := hex.EncodeToString(bytes)

		loggerReq := log.With().
			Str("user_id", userID).
			Str("user_ip", userIP).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("role", userRole).
			Str("trace_id", trace).Logger()

		// Cute replica:
		//     type contextKey string
		//     const zerologCtxKey contextKey = "userLogger"
		//     ctx := context.WithValue(r.Context, zerologCtxKey, loggerReq)
		// Why I pointed this out? No exact reason - just cute enough to nitpick around
		ctx := loggerReq.WithContext(r.Context())
		// We pass the trace here, because Logger is global (used by registration and login too)
		ctx = context.WithValue(ctx, traceCtx, trace)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type RequestContext struct {
	ID    pgtype.UUID
	Role  db.UserRole
	Trace string
}

// We create multiple functions to return varuious values, because we cannot fetch it somewhere else (because the type is not a string, but ctx thingy)
func getID(ctx context.Context) (pgtype.UUID, bool) {
	userID, ok := ctx.Value(idCtx).(pgtype.UUID)
	return userID, ok
}

func getRole(ctx context.Context) (db.UserRole, bool) {
	userRole, ok := ctx.Value(roleCtx).(db.UserRole)
	return userRole, ok
}

func GetTrace(ctx context.Context) (string, bool) {
	userTrace, ok := ctx.Value(traceCtx).(string)
	return userTrace, ok
}

// We delete redundancy by using a simple function
func FetchContext(w http.ResponseWriter, r *http.Request) (RequestContext, bool) {
	logger := zerolog.Ctx(r.Context()).With().Str("handler", "FetchContext").Logger()
	trace, ok := GetTrace(r.Context())
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
		return RequestContext{}, false
	}

	id, ok := getID(r.Context())
	if !ok {
		logger.Warn().
			Int("status", http.StatusUnauthorized).
			Str("cause", "missing_id_context").
			Str("trace_id", trace).
			Msg("unauthenticated request attempt")

		lib.Pretty(w, http.StatusUnauthorized, lib.Error{
			Code:    "UNAUTHORIZED",
			Message: "Authentication required to access this resource",
			Details: map[string]string{
				"reason": "user identity missing from request context",
				"fix":    "Include a valid 'Authorization: Bearer <token>' header",
			},
			TraceID: trace,
		})
		return RequestContext{}, false
	}

	role, ok := getRole(r.Context())
	if !ok {
		logger.Warn().
			Int("status", http.StatusForbidden).
			Str("cause", "missing_role_context").
			Str("user_id", id.String()).
			Str("trace_id", trace).
			Msg("user role context missing or invalid")

		lib.Pretty(w, http.StatusForbidden, lib.Error{
			Code:    "FORBIDDEN",
			Message: "User permissions could not be verified",
			Details: map[string]string{
				"reason": "user role missing from context",
			},
			TraceID: trace,
		})
		return RequestContext{}, false
	}

	return RequestContext{
		ID:    id,
		Role:  role,
		Trace: trace,
	}, true
}
