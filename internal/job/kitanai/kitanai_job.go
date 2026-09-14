package kitanaijob

import (
	"context"
	db "prolepsis/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// Easy workflow explained so I don't forget (a simple cheatsheet)
// Inside the "*_job.go" files we simply declare the startup of River:
// -- strcutArgs -> A simple structure where we declare what exactly we need for the future (since sqlc.DeleteUser needs the id, we declared a simple id)
// -- Kind() -> We attached it as a method of "structArgs" - when calling "structArgs" river will automatically run "Kind".
// -- -- Per se, Kind, is a simple job identifier (it simply names the job, that's all, it doesn't execute or do nothing with it)
// -- structWork -> Here we declare the "structArgs" and whatever we need else, what will run as example (in our case, we want it to run an sqlc command, so we default to "*db.Queries")
// -- Work() -> This is the actual executioner of our workflow, here we declared that we want it to run an exact command. If "structWork" can be considered the blueprint, this is the actual worker.
type JobHandler struct {
	RiverClient *river.Client[pgx.Tx]
	Queries     *db.Queries
}

func New(client *river.Client[pgx.Tx], queries *db.Queries) *JobHandler {
	return &JobHandler{
		RiverClient: client,
		Queries:     queries,
	}
}

// How this function works in easy (or simple, whatever) words:
// -- It simply created a new worker (it's empty, since it's new) - think about the "workers" as an empty directory (or folder, same stuff)
// -- Then we add something in that "empty folder", via the command "AddWorker" (we added DeleteUserWork -> which has as method "Work", which River will execute under the hood), but this after we set the timer and stuff)
// -- When creating the worker, we provide the "queries" key a value (which is j.Queries), everything that JobHandler will have as value for Queries, will be the value for the "queries" key inside DeleteUserArgs
func SetupDeletionWorkers(queries *db.Queries) *river.Workers {
	workers := river.NewWorkers()

	river.AddWorker(workers, &DeleteUserWork{
		queries: queries,
	})

	return workers
}

// Easy explanation of how it works:
// -- Here we starta adding data (the ID) to "DeleteUserArgs" (meaning that we are a step closer to set the alarm)
// -- Since the id is already added, we simply add when it should be executed (usually based on the scheduled_at row inside "pending_registrations")
func (j *JobHandler) SetupDeletionAlarm(ctx context.Context, scheduled_at pgtype.Timestamptz, userID pgtype.UUID) error {
	_, err := j.RiverClient.Insert(ctx, DeleteUserArgs{
		UserID: userID,
	}, &river.InsertOpts{
		ScheduledAt: scheduled_at.Time,
	})
	if err != nil {
		return err
	}

	return nil
}
