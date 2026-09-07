-- We use the token to fetch the info about the user
-- name: CheckToken :one
SELECT users.id, users.role 
FROM sessions 
JOIN users ON users.id = sessions.user_id 
WHERE sessions.token = $1 AND sessions.expires_at > now();

-- Simply insert the opaque token
-- name: CreateSession :exec
INSERT INTO sessions(user_id, token, created_at, expires_at)
VALUES (sqlc.arg('user_id'), sqlc.arg('token'), now(), now() + interval '60 days');

-- We update the sessions if the user is active (we won't like to expire the sessions of an active user, no?)
-- name: UpdateSession :exec
UPDATE sessions
SET expires_at = now() + interval '60 days'
-- We update the token lifetime only if it's older than 30 days
WHERE token = $1 AND expires_at > now() AND expires_at < now() + interval '30 days';

-- We simply delete the sessions from users whose token is expired or when they log out
-- name: DeleteSession :exec
DELETE FROM sessions
WHERE token = $1;