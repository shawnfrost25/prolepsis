-- We store the request to delete a user inside the database
-- name: InsertDeletionRequest :exec
INSERT INTO pending_deletions (user_id, requested_by, clarification, requested_at, is_processed)
VALUES (
    sqlc.arg('user_id')::uuid,
    sqlc.arg('requested_by')::uuid,
    sqlc.arg('clarification')::text,
    now(),
    false
);

-- We delete away the user
-- name: DeleteUserByRequest :exec
DELETE FROM users
WHERE id = (
    SELECT id FROM pending_deletions WHERE id = sqlc.arg('user_id')::uuid AND is_processed = false
);