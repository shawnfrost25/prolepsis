package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	db "prolepsis/internal/db/sqlc"
	"prolepsis/internal/errlog"
	"prolepsis/internal/lib"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type contextKey string

const traceCtx contextKey = "userTrace"
const idCtx contextKey = "userID"
const roleCtx contextKey = "userRole"
const ipCtx contextKey = "userIP"
const methodCtx contextKey = "userMethod"
const pathCtx contextKey = "userPath"
const timeCtx contextKey = "timeStarted"
const durationCtx contextKey = "durationCtx"

var queries *db.Queries
var red *redis.Client

// Workflow without this function:
// We delcare 'queries' (a placeholder) -> function starts for real -> no value given to 'queries' -> defaults to 'nil' (because it has a pointer, it's '*db.Queries')
func Init(q *db.Queries, rb *redis.Client) {
	queries = q
	red = rb
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

		timeNow := time.Now()

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

		id, err := red.Get(timeout, "session:token:"+hashToken).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Couldn't fetch token due to timeout",
				})
				return
			}
			if errors.Is(err, redis.Nil) {
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
		var uuid pgtype.UUID
		err = uuid.Scan(id)
		if err != nil {
			log.Error().Err(err).Str("id_string", id).Msg("Failed to parse UUID from Redis string")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Session contains an invalid identifier data format",
			})
			return
		}

		beforeExpirationT, err := red.TTL(timeout, "session:token:"+hashToken).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Couldn't obtain token TTL due to timeout",
				})
				return
			}
			log.Error().Err(err).Msg("CheckToken failed")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while fetching the TTL of the token",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
			})
			return
		}

		if beforeExpirationT < 2592000*time.Second {
			_, err := red.Expire(timeout, "session:token:"+hashToken, 5184000*time.Second).Result()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					lib.Pretty(w, http.StatusInternalServerError, lib.Error{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "Couldn't update the token due to timeout",
					})
					return
				}
				log.Error().Err(err).Msg("Token update failed")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "An error occurred while fetching the TTL of the token",
					Details: map[string]string{
						"reason": "database error",
						"fix":    "Please try again later",
					},
				})
				return
			}
		}
		beforeExpirationID, err := red.TTL(timeout, "session:id:"+uuid.String()).Result()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Couldn't obtain token TTL due to timeout",
				})
				return
			}
			log.Error().Err(err).Msg("CheckToken failed")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "An error occurred while fetching the TTL of the token",
				Details: map[string]string{
					"reason": "database error",
					"fix":    "Please try again later",
				},
			})
			return
		}

		if beforeExpirationID < 2592000*time.Second {
			_, err := red.Expire(timeout, "session:id:"+uuid.String(), 5184000*time.Second).Result()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					lib.Pretty(w, http.StatusInternalServerError, lib.Error{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "Couldn't update the token due to timeout",
					})
					return
				}
				log.Error().Err(err).Msg("Token update failed")
				lib.Pretty(w, http.StatusInternalServerError, lib.Error{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "An error occurred while fetching the TTL of the token",
					Details: map[string]string{
						"reason": "database error",
						"fix":    "Please try again later",
					},
				})
				return
			}
		}

		info, err := queries.GetUserByID(timeout, uuid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				log.Error().Err(err).Msg("no users match the provided id")
				lib.Pretty(w, http.StatusNotFound, lib.Error{
					Code:    "NOT_FOUND",
					Message: "The provided id is invalid",
					Details: map[string]string{
						"reason": "the provided id doesn't matches nobody in the database",
						"fix":    "try logging out and logging in again",
					},
				})
				return
			}
			if errors.Is(err, pgconn.ErrConnClosed) {
				log.Error().Err(err).Msg("timeout while searching an user matching the provided id")
				lib.Pretty(w, http.StatusRequestTimeout, lib.Error{
					Code:    "REQUEST_TIME",
					Message: "Timeout while querying through the users to match the provided id",
				})
				return
			}
			log.Error().Err(err).Msg("unrecognized error while querying provided id")
			lib.Pretty(w, http.StatusInternalServerError, lib.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Unrecognized error while querying the id",
			})
			return
		}

		ctx := context.WithValue(r.Context(), idCtx, uuid)

		ctx = context.WithValue(ctx, roleCtx, info.Role)
		ctx = context.WithValue(ctx, timeCtx, timeNow)
		ctx = context.WithValue(ctx, durationCtx, time.Since(timeNow))
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
		timeNow := time.Now()

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
			Time("started_at", timeNow).
			Str("trace_id", trace).Logger()

		ctx := loggerReq.WithContext(r.Context())
		ctx = context.WithValue(ctx, traceCtx, trace)

		next.ServeHTTP(w, r.WithContext(ctx))

		duration := time.Since(timeNow)

		loggerReq.Info().Dur("duration", duration).Msg("request finished")
	})
}

type RequestContext struct {
	ID     pgtype.UUID
	IP     string
	Method string
	Role   db.UserRole
	Trace  string
	Path   string
}

// We create multiple functions to return varuious values, because we cannot fetch it somewhere else (because the type is not a string, but ctx thingy)
func GetID(ctx context.Context) (pgtype.UUID, bool) {
	userID, ok := ctx.Value(idCtx).(pgtype.UUID)
	return userID, ok
}

func GetRole(ctx context.Context) (db.UserRole, bool) {
	userRole, ok := ctx.Value(roleCtx).(db.UserRole)
	return userRole, ok
}

func GetTrace(ctx context.Context) (string, bool) {
	userTrace, ok := ctx.Value(traceCtx).(string)
	return userTrace, ok
}

func GetIP(ctx context.Context) (string, bool) {
	userIP, ok := ctx.Value(ipCtx).(string)
	return userIP, ok
}

func GetMethod(ctx context.Context) (string, bool) {
	userMethod, ok := ctx.Value(methodCtx).(string)
	return userMethod, ok
}

func GetPath(ctx context.Context) (string, bool) {
	userPath, ok := ctx.Value(pathCtx).(string)
	return userPath, ok
}

// We delete redundancy by using a simple function
func FetchContextInsideHandler(w http.ResponseWriter, r *http.Request) (RequestContext, bool) {
	logger := zerolog.Ctx(r.Context()).With().Str("op", "fetch_context").Logger()
	trace, ok := GetTrace(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "trace", w, trace)
		return RequestContext{}, false
	}

	id, ok := GetID(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "id", w, trace)
		return RequestContext{}, false
	}

	role, ok := GetRole(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "role", w, trace)
		return RequestContext{}, false
	}

	ip, ok := GetIP(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "ip", w, trace)
		return RequestContext{}, false
	}

	path, ok := GetPath(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "path", w, trace)
		return RequestContext{}, false
	}

	method, ok := GetMethod(r.Context())
	if !ok {
		errlog.EmptyContextError(logger, "method", w, trace)
		return RequestContext{}, false
	}

	return RequestContext{
		ID:     id,
		IP:     ip,
		Method: method,
		Role:   role,
		Trace:  trace,
		Path:   path,
	}, true
}
