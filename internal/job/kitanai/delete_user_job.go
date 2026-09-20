package kitanaijob

import (
	"context"
	db "prolepsis/internal/db/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type DeleteUserArgs struct {
	UserID pgtype.UUID
}

func (DeleteUserArgs) Kind() string {
	return "delete_user"
}

type DeleteUserWork struct {
	river.WorkerDefaults[DeleteUserArgs]
	queries *db.Queries
}

func (w DeleteUserWork) Work(ctx context.Context, job *river.Job[DeleteUserArgs]) error {
	err := w.queries.DeleteUser(ctx, job.Args.UserID)
	if err != nil {
		return err
	}
	// We mark the deleted users as "is_processed = true" - so we know who is deleted
	return w.queries.UpdateIsProcessedField(ctx, job.Args.UserID)
}
