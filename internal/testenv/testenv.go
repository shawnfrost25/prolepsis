package testenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	db "prolepsis/internal/db/sqlc"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Executes "start" // we use it so we return and exit code (also to make defer run, since via exit they don't)
func Main(m *testing.M) {
	os.Exit(start(m))
}

// Here "start" will give to each a value, so they become usable inside "New()" <- That will give us the necessary components for "Server" structure
var (
	rdb *redis.Client
	dsn string
	pg  *postgres.PostgresContainer
)

// Explanation of how it works:
// -- Phase 1:
// -- -- The first phase is relatively simple: it uses 'filepath' and 'runtime.Caller' to resolve the current path (prolepsis/internal/testenv/testenv.go), storing that path inside the variable 'thisFile'.
// -- -- Then we use filepath.Dir(thisFile) to strip away the filename, leaving only the enclosing directory path (prolepsis/internal/testenv), which we store inside 'dir'.
// -- -- After that, we use filepath.Join(...) to specify the precise directory location of our migration files.
// -- -- Finally, we fetch all matching migration files from that directory using filepath.Glob.
// -- Phase 2:
// -- -- We invoke postgres.Run to launch a Docker container with the "postgres:18-alpine" image, automatically executing all fetched migration files upon startup.
// -- -- We store the container handle 'pgc' inside the package-level variable 'pg'.
// -- -- Then we use 'pgc' to generate 'dsnPgc' with all container connection parameters (formatted as 'postgres://test:test@localhost:49153/prolepsis_test?sslmode=disable').
// -- -- We then store 'dsnPgc' inside the package-level variable 'dsn'.
// -- -- Finally, we capture a snapshot of the current database state—functioning like a game save-point so we can restore back to a clean state before each test.
// -- Phase 3:
// -- -- We replicate the container lifecycle workflow for Redis (starting the container, retrieving its connection URI, and configuring the client).
// -- -- Finally, we assign the *redis.Client to the package-level variable 'rdb' and execute all tests by returning 'm.Run()' (which evaluates to 0 if every test passes, or non-zero on failure).
func start(m *testing.M) int {
	// Phase 1:
	ctx := context.Background()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintf(os.Stderr, "testenv: failed to get current path\n")
		return 1
	}

	dir := filepath.Dir(thisFile)
	pattern := filepath.Join(dir, "..", "db", "migrations", "*.sql")

	migrationFiles, err := filepath.Glob(pattern)
	if err != nil || len(migrationFiles) == 0 {
		fmt.Fprintf(os.Stderr, "testenv: missing or invalid migration files at %s: %v\n", pattern, err)
		return 1
	}

	// ----------
	// Phase 2:
	pgc, err := postgres.Run(
		ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("prolepsis_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(migrationFiles...),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to start postgres container: %v\n", err)
		return 1
	}

	pg = pgc
	defer func() {
		if err := pgc.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "testenv: failed to terminate postgres container: %v\n", err)
		}
	}()

	dsnPgc, err := pgc.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to get database dsn: %v\n", err)
		return 1
	}
	dsn = dsnPgc

	if err := pgc.Snapshot(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to snapshot database: %v\n", err)
		return 1
	}

	// ----------
	// Phase 3:
	rd, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to start redis container: %v\n", err)
		return 1
	}

	defer func() {
		if err := rd.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "testenv: failed to terminate redis container: %v\n", err)
		}
	}()

	rdURI, err := rd.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to get redis uri\n")
		return 1
	}

	redOpt, err := redis.ParseURL(rdURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: failed to parse redis uri: %v\n", err)
		return 1
	}

	rdb = redis.NewClient(redOpt)

	return m.Run()
}

// We begin by restoring the latest database snapshot, ensuring a pristine schema and clean state before each test executes.
// Next, we invoke 'rdb.FlushAll' to purge all Redis entries, guaranteeing a fully cleared key-value cache prior to the next test.
// Finally, we register 't.Cleanup()' to automatically close the database pool upon test completion, returning all connection handles required to construct the Server struct.
func New(t *testing.T) (*redis.Client, *pgxpool.Pool, *db.Queries) {
	t.Helper()

	if pg == nil {
		t.Fatal("testenv: call testenv.Main(m) from TestMain first")
	}

	ctx := context.Background()
	if err := pg.Restore(ctx); err != nil {
		t.Fatalf("testenv: failed to restore database snapshot: %v", err)
	}

	if err := rdb.FlushAll(ctx).Err(); err != nil {
		t.Fatalf("testenv: failed to flush redis: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testenv: failed to create pgxpool: %v", err)
	}

	queries := db.New(pool)

	t.Cleanup(pool.Close)

	return rdb, pool, queries
}
