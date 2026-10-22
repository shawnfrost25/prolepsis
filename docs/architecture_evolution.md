## Logging evolution
* **What to expect:**
* * This section explains how my logging evolved from basic inline messages into a centralized error-handling and logging system.
* * It also includes the problems that appeared during the process, because the architecture was not designed perfectly from the beginning. Each phase was a response to issues in the previous one.

---

### Phase 1
* **What was it:**
* * This was the early-development logging style.
* * The goal was simple: when something failed, I wanted more than the raw error. I added a status code, a rough cause, and a short message.
* * The logging was written directly inside the handler, next to the validation or operation that could fail.
* **Where to find:**
* * Commit [`a0a706d`](https://github.com/shawnfrost25/prolepsis/commit/a0a706db49ba7aabfac86c864d4c96362bd1bf0d) -> feat(api): add auth middleware, user handlers, and schema updates
* **How it looked inside the code:**
* * A handler (`internal/handler/kitanai/login_user.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/a0a706db49ba7aabfac86c864d4c96362bd1bf0d/internal/handler/kitanai/login_user.go)) <- deprecated, logged like this:
    ```go
    if id == "" {
    	logger.Warn().
    		Int("status", http.StatusBadRequest).
    		Str("cause", "missing_id_parameter").
    		Msg("invalid request")
    }
    ```
* **Pros:**
* * **Simple** -> It was easy to add a log wherever something failed.
* * **Immediate visibility** -> I could quickly see the status, cause, and message.
* * **Low abstraction** -> There was no extra system to understand before logging an error.
* **Cons:**
* * **Inconsistent** -> It was extremely easy to create different names for the same failure. One place could use `"missing_id_parameter"` while another used `"registration_parameter_missing"`.
* * **Redundant** -> The same logging fields and HTTP response logic were repeated throughout the handlers.
* * **Vague** -> Most logs only contained a status, a cause, and a shallow message.
* * **Hard to maintain** -> Changing the format or response behavior required manually editing many handlers.
* * **Weak error context** -> The log did not clearly explain which operation, subsystem, or application flow had failed.

---

### Phase 2
* **What was it:**
* * This was the first major architectural step.
* * Instead of writing every error log directly inside the handler, I started placing specific error behaviors into functions inside an `errlog` package.
* * Each function represented a particular type of failure, such as:
* * * `EmptyValueError`
* * * `ParsingError`
* * * `DecodeError`
* * * `TimeoutError`
* * * `NoRowsError`
* * * `ErrConnClosed`
* * * `PgConnError`
* * * `UnexpectedError`
* * The handler would call the specific error function instead of repeatedly writing the entire logging and response block.
* **Where to find:**
* * Commit [`22aab53`](https://github.com/shawnfrost25/prolepsis/commit/22aab53f2cfa73bc9791fc361b00a7480a43a289) -> feat(errlog): add structured error logging helpers
* **How it looked inside the code:**
* * A validation helper (`internal/errlog/validate.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/22aab53f2cfa73bc9791fc361b00a7480a43a289/internal/errlog/validate.go)) looked like this:
    ```go
    func EmptyValueError(
    	logger zerolog.Logger,
    	subject string,
    	w http.ResponseWriter,
    	trace string,
    ) {
    	subjectMsg := strings.ReplaceAll(subject, "_", " ")
    
    	logger.Warn().
    		Int("status", http.StatusBadRequest).
    		Str("code", "missing_"+subject).
    		Msg(subjectMsg + " is missing")
    
    	lib.Pretty(w, http.StatusBadRequest, lib.Error{
    		Code:    "BAD_REQUEST",
    		Message: "Missing parameter",
    		Details: map[string]string{
    			"reason": "a specific value was not provided",
    		},
    		TraceID: trace,
    	})
    }
    ```
* * A database error received its own helper as well (`internal/errlog/database.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/22aab53f2cfa73bc9791fc361b00a7480a43a289/internal/errlog/database.go)):
    ```go
    func PgConnError(
    	logger zerolog.Logger,
    	w http.ResponseWriter,
    	trace string,
    	code string,
    	pgErr *pgconn.PgError,
    ) {
    	logger.Error().
    		Int("status", http.StatusInternalServerError).
    		Str("code", code).
    		Str("database_constraint", pgErr.ConstraintName).
    		Str("sqlstate", pgErr.Code).
    		Str("database_detail", pgErr.Detail).
    		Str("database_hint", pgErr.Hint).
    		Msg("database failed")
    
    	lib.Pretty(w, http.StatusInternalServerError, lib.Error{
    		Code:    "INTERNAL_SERVER_ERROR",
    		Message: "An internal error occurred",
    		TraceID: trace,
    	})
    }
    ```
* * The handler no longer needed to repeat the entire block:
    ```go
    errlog.EmptyValueError(
    	logger,
    	"user_id",
    	w,
    	trace,
    )
    ```
* **Pros:**
* * **Centralized formatting** -> The log format and response format were moved away from the handler.
* * **Reusable failures** -> A common failure could now be reused through a function.
* * **Consistent responses** -> The HTTP response was generated consistently by the error helper.
* * **Richer logs** -> The logger could include more information, such as database constraints, SQL states, details, and hints.
* * **Smaller handlers** -> The handlers became slightly smaller because they delegated repeated error behavior.
* **Cons:**
* * **Too many specific functions** -> Every small variation required another function.
* * **Function duplication** -> Several functions repeated almost the same structure:
* * * Create a human-readable subject.
* * * Call `logger.Warn()` or `logger.Error()`.
* * * Attach a status.
* * * Attach a code.
* * * Call `lib.Pretty(...)`.
* * **The handler still had to choose the exact function** -> The calling code had to know whether an error was a `NoRowsError`, `ErrConnClosed`, `PgConnError`, `ParsingError`, or `UnexpectedError`.
* * **Inconsistent behavior could still happen** -> Different helper functions could return slightly different error messages or use different code naming conventions.
* * **The architecture was organized by individual cases rather than error categories** -> Instead of one general validation or database flow, the package was becoming a collection of separate functions.
* **Takeaway:**
* * This phase was a significant improvement over Phase 1, but it also exposed a new problem: the duplication was no longer mainly inside the handlers. It had moved into the `errlog` package.

---

### Phase 3
* **What was it:**
* * This phase focused on polishing and reorganizing the helper-based system.
* * Instead of continuing to create one function for every individual error case, I started grouping errors by responsibility.
* * New categories appeared around:
* * * authentication,
* * * internal errors,
* * * resources,
* * * validation,
* * * JSON parsing,
* * * server errors.
* * Some old helpers were removed or reshaped because their behavior overlapped with other helpers.
* * The logging output was also made more consistent.
* **Where to find:**
* * Commit [`063a34b`](https://github.com/shawnfrost25/prolepsis/commit/063a34b7a67adce9ed4963e0ec026d9772d68579) -> refactor(errlog): improve log structure and output
* **How it looked inside the code:**
* * A validation helper (`internal/errlog/http.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/063a34b7a67adce9ed4963e0ec026d9772d68579/internal/errlog/http.go)) now looked like this:
    ```go
    func Validate(
    	logger zerolog.Logger,
    	w http.ResponseWriter,
    	subject string,
    	kind failure.KindType,
    	err error,
    	trace string,
    ) {
    	respond := func(
    		status int,
    		errorCode string,
    		message string,
    	) {
    		lib.Pretty(w, status, lib.Error{
    			Code:    errorCode,
    			Message: message,
    			TraceID: trace,
    		})
    	}
    
    	subjectMsg := strings.ReplaceAll(subject, "_", " ")
    
    	switch kind {
    	case failure.EmptyValueError:
    		logger.Warn().
    			Err(err).
    			Int("status", http.StatusBadRequest).
    			Str("code", subject+"_missing").
    			Msg(subjectMsg + " is missing")
    
    		respond(
    			http.StatusBadRequest,
    			"BAD_REQUEST",
    			"A required field is missing",
    		)
    
    	case failure.MalformedError:
    		logger.Warn().
    			Err(err).
    			Int("status", http.StatusBadRequest).
    			Str("code", subject+"_malformed").
    			Msg("resource is malformed")
    
    		respond(
    			http.StatusBadRequest,
    			"BAD_REQUEST",
    			"The provided data is malformed",
    		)
    
    	case failure.InvalidError:
    		logger.Warn().
    			Err(err).
    			Int("status", http.StatusBadRequest).
    			Str("code", subject+"_invalid").
    			Msg("request validation failed")
    
    		respond(
    			http.StatusBadRequest,
    			"BAD_REQUEST",
    			"The given resource does not comply with the requirements",
    		)
    	}
    }
    ```
* * The main difference was that validation behavior was now selected through a type:
    ```go
    kind failure.KindType
    ```
* * Instead of calling a different function for every validation situation, the system could call one validation helper and provide the specific kind of failure.
* * The same approach was applied to internal errors (`internal/errlog/internal.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/063a34b7a67adce9ed4963e0ec026d9772d68579/internal/errlog/internal.go)):
    ```go
    func Internal(
    	logger zerolog.Logger,
    	w http.ResponseWriter,
    	subject string,
    	kind failure.KindType,
    	err error,
    	trace string,
    ) {
    	subjectMsg := strings.ReplaceAll(subject, "_", " ")
    
    	switch kind {
    	case failure.InternalEmptyValueError:
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusInternalServerError).
    			Str("code", subject+"_missing").
    			Msg(subjectMsg + " is missing")
    
    		InternalRes(w, trace)
    
    	case failure.GenerationError:
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusInternalServerError).
    			Str("code", subject+"_generation_failed").
    			Msg("failed to generate " + subjectMsg)
    
    		InternalRes(w, trace)
    	}
    }
    ```
* * JSON errors were also moved into their own category and began extracting more specific information (`internal/errlog/json.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/063a34b7a67adce9ed4963e0ec026d9772d68579/internal/errlog/json.go)):
    ```go
    synErr, isSyntaxError :=
    	errors.AsType[*json.SyntaxError](err)
    
    unmTypeErr, isUnmTypeErr :=
    	errors.AsType[*json.UnmarshalTypeError](err)
    
    switch {
    case isSyntaxError:
    	logger.Warn().
    		Err(err).
    		Int("status", http.StatusBadRequest).
    		Int64("offset", synErr.Offset).
    		Str("code", subject+"_payload_syntax_error").
    		Msg("invalid json payload")
    
    case isUnmTypeErr:
    	logger.Warn().
    		Err(err).
    		Int("status", http.StatusBadRequest).
    		Str("field", unmTypeErr.Field).
    		Str("given_type", unmTypeErr.Value).
    		Str("expected_type", unmTypeErr.Type.String()).
    		Int64("offset", unmTypeErr.Offset).
    		Str("code", subject+"_payload_type_error").
    		Msg("invalid json type")
    }
    ```
* **Pros:**
* * **Less duplicated behavior** -> Similar validation errors could now be handled by one helper.
* * **Clearer categories** -> Validation, internal errors, JSON errors, authentication errors, and resource errors started having their own responsibilities.
* * **Better structured fields** -> JSON logs could include the field, received type, expected type, and parsing offset.
* * **More consistent output** -> The log messages and response shapes became more standardized.
* * **Better separation** -> The application could distinguish between a client error and an internal server error more clearly.
* **Cons:**
* * **The abstraction became more complicated** -> It was no longer enough to call a simple function; the developer now had to understand `failure.KindType`.
* * **Some files started overlapping** -> Error responsibilities were still being moved around between `validate.go`, `internal.go`, `resource.go`, `server.go`, and the newer helpers.
* * **The organization was still transitional** -> Some functions were added while older functions were modified or later removed.
* * **The output was still being refined** -> Some fields and messages changed during this phase as the structure became clearer.
* **Takeaway:**
* * This phase was less about adding more logging and more about trying to turn the growing collection of helper functions into a coherent error taxonomy.

---

### Phase 4
* **What was it:**
* * This phase was the largest consolidation of the logging system.
* * The many individual error functions were replaced with broader subsystem-level helpers.
* * Instead of having separate functions for every individual database, Redis, validation, or JSON case, the helper would inspect the error and classify it internally.
* * The logging architecture became closer to its current form:
* * * one database helper,
* * * one Redis helper,
* * * one JSON helper,
* * * one HTTP validation helper,
* * * one general error helper,
* * * one internal error helper.
* * The handler would provide the logger, subject, trace, and error; the helper would determine the specific result.
* **Where to find:**
* * Commit [`097ef79`](https://github.com/shawnfrost25/prolepsis/commit/097ef799c5e7de643e42d96e34232ed81eeb4302) -> refactor(errlog): improve logging structure and performance
* * Commit [`713bcda`](https://github.com/shawnfrost25/prolepsis/commit/713bcda1afb9cc6714bbf6786ec16a45e22eabbf) -> feat(failure): add error context helper
* **Database classification:**
* * Before this change, the caller had to decide which database helper to invoke.
* * Afterward, the database helper became responsible for inspecting the error (`internal/errlog/database.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/097ef799c5e7de643e42d96e34232ed81eeb4302/internal/errlog/database.go)):
    ```go
    func Database(
    	logger zerolog.Logger,
    	w http.ResponseWriter,
    	subject string,
    	trace string,
    	err error,
    ) {
    	respond := func(
    		status int,
    		errorCode string,
    		message string,
    	) {
    		lib.Pretty(w, status, lib.Error{
    			Code:    errorCode,
    			Message: message,
    			TraceID: trace,
    		})
    	}
    
    	pgErr, isPgErr :=
    		errors.AsType[*pgconn.PgError](err)
    
    	switch {
    	case errors.Is(err, context.DeadlineExceeded):
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusGatewayTimeout).
    			Str("code", "database_timeout").
    			Msg("database operation timed out")
    
    		respond(
    			http.StatusGatewayTimeout,
    			"TIMEOUT",
    			"The operation timed out",
    		)
    
    	case errors.Is(err, pgconn.ErrConnClosed):
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusInternalServerError).
    			Str("code", "database_connection_closed").
    			Msg("database connection closed")
    
    		InternalRes(w, trace)
    
    	case errors.Is(err, pgx.ErrNoRows):
    		logger.Warn().
    			Err(err).
    			Int("status", http.StatusNotFound).
    			Str("code", subject+"_not_found").
    			Msg("nothing matches the provided resource")
    
    		respond(
    			http.StatusNotFound,
    			"NOT_FOUND",
    			"Nothing matches the provided resource",
    		)
    
    	case isPgErr:
    		switch pgErr.SQLState() {
    		case "23505":
    			logger.Warn().
    				Err(err).
    				Int("status", http.StatusConflict).
    				Str("sqlstate", pgErr.SQLState()).
    				Str("constraint", pgErr.ConstraintName).
    				Str("schema", pgErr.SchemaName).
    				Str("table", pgErr.TableName).
    				Str("code", subject+"_already_exists").
    				Msg("insert conflict")
    
    			respond(
    				http.StatusConflict,
    				"CONFLICT",
    				"A resource with the provided identifier already exists",
    			)
    
    		case "40P01":
    			logger.Error().
    				Err(err).
    				Int("status", http.StatusServiceUnavailable).
    				Str("sqlstate", "40P01").
    				Str("code", subject+"_deadlock").
    				Msg("database deadlock detected")
    
    			respond(
    				http.StatusServiceUnavailable,
    				"SERVICE_UNAVAILABLE",
    				"The operation could not be completed temporarily",
    			)
    		}
    	}
    }
    ```
* * The caller could now simply do:
    ```go
    err = failure.Wrap(
    	"kitanai.database.user.fetch_via_email",
    	err,
    )
    
    errlog.Database(
    	logger,
    	w,
    	"user_lookup",
    	trace,
    	err,
    )
    ```
* * The helper handled the difference between:
* * * timeout,
* * * no rows,
* * * closed connection,
* * * unique conflict,
* * * foreign-key violation,
* * * deadlock,
* * * serialization failure,
* * * connection exhaustion,
* * * unexpected database failure.
* **Redis classification:**
* * The same consolidation happened for Redis (`internal/errlog/redis.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/097ef799c5e7de643e42d96e34232ed81eeb4302/internal/errlog/redis.go)):
    ```go
    func Redis(
    	logger zerolog.Logger,
    	w http.ResponseWriter,
    	subject string,
    	err error,
    	trace string,
    ) {
    	respond := func(
    		status int,
    		errorCode string,
    		message string,
    	) {
    		lib.Pretty(w, status, lib.Error{
    			Code:    errorCode,
    			Message: message,
    			TraceID: trace,
    		})
    	}
    
    	red, isRedisError :=
    		errors.AsType[redis.Error](err)
    
    	switch {
    	case errors.Is(err, context.DeadlineExceeded):
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusGatewayTimeout).
    			Str("code", "redis_timeout").
    			Msg("redis operation timed out")
    
    		respond(
    			http.StatusGatewayTimeout,
    			"TIMEOUT",
    			"The operation timed out",
    		)
    
    	case errors.Is(err, redis.Nil):
    		logger.Warn().
    			Err(err).
    			Int("status", http.StatusNotFound).
    			Str("code", subject+"_not_found").
    			Msg("expired or nonexistent redis entry")
    
    		respond(
    			http.StatusNotFound,
    			"NOT_FOUND",
    			"Nothing matches the provided resource",
    		)
    
    	case isRedisError:
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusInternalServerError).
    			Str("code", subject+"_failed").
    			Msg("redis server rejected command: " + red.Error())
    
    		InternalRes(w, trace)
    
    	default:
    		logger.Error().
    			Err(err).
    			Int("status", http.StatusInternalServerError).
    			Str("code", subject+"_failed").
    			Msg("unexpected redis error")
    
    		InternalRes(w, trace)
    	}
    }
    ```
* **Error context and typed failure kinds:**
* * Commit `713bcda` introduced a more formal error vocabulary (`internal/failure/kind.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/713bcda1afb9cc6714bbf6786ec16a45e22eabbf/internal/failure/kind.go)):
    ```go
    type KindType string
    
    const (
    	EmptyValueError         KindType = "empty_value"
    	InternalEmptyValueError KindType = "internal_empty_value"
    	MalformedError          KindType = "malformed"
    	InvalidError            KindType = "invalid"
    	GenerationError         KindType = "generation"
    	UnauthorizedError       KindType = "unauthorized"
    	ForbiddenError          KindType = "forbidden"
    	NotFoundError           KindType = "not_found"
    	ConflictError           KindType = "conflict"
    	GeneralError            KindType = "general"
    	InternalGeneralError    KindType = "internal_general"
    )
    ```
* * This made errors more explicit. Instead of relying only on function names or string codes, an error could now have a known application-level category.
* * The same commit also introduced the idea of wrapping errors with their application operation (`internal/failure/wrap.go`, [view on GitHub](https://github.com/shawnfrost25/prolepsis/blob/713bcda1afb9cc6714bbf6786ec16a45e22eabbf/internal/failure/wrap.go)):
    ```go
    func Wrap(process string, err error) error {
    	if err != nil {
    		return nil
    	}
    
    	return fmt.Errorf("%s: %w", process, err)
    }
    ```
* * This was an important architectural direction because it attempted to preserve both:
* * * the original underlying error,
* * * the application process where the error occurred.
* * It also shows one of the imperfect moments in the evolution: the initial helper contains a logic mistake, since it returns `nil` when an error is present. That is part of the journey - the design direction was correct, but the first implementation still needed correction.
* **Pros:**
* * **Large reduction in repetition** -> Handlers no longer needed to know every possible infrastructure-specific error helper.
* * **Centralized classification** -> The database helper understood PostgreSQL errors, and the Redis helper understood Redis errors.
* * **Better error context** -> Wrapped errors could describe the application operation, such as `"kitanai.database.user.fetch_via_email"`.
* * **More consistent responses** -> The same underlying error type produced the same status, code, and public message.
* * **Better extensibility** -> New database or Redis cases could be added inside the subsystem helper.
* * **Cleaner handlers** -> The handler mainly described what operation failed and delegated classification to `errlog`.
* * **More meaningful structured logs** -> Logs could include SQL states, constraints, tables, fields, operation paths, traces, and subsystem-specific information.
* **Cons:**
* * **The helpers became more complex** -> `errlog.Database` and `errlog.Redis` now contain substantial classification logic.
* * **Debugging required understanding multiple layers** -> It could mean following the handler, `failure.Wrap`, the failure kind, and the subsystem helper.
* * **Some migration problems appeared** -> Older helper names and newer consolidated helpers existed during the transition.
* * **The abstraction could hide details** -> A handler calling `errlog.Database(...)` is cleaner, but the exact response behavior is no longer visible locally.