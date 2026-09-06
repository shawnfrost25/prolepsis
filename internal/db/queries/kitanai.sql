-- An SQL code to fetch user information based on the given ID
-- name: GetUserByID :one
SELECT name, display_name, bio, sex, location, birth_date, email, role, created_at
FROM users
WHERE id = $1;

-- An SQL code to add info insdie the not-forced fields (display_name, bio, location)
-- Update rows in 'TableName' where condition is met
-- name: UpdateUserNotForced :exec
UPDATE users
SET 
display_name = COALESCE(sqlc.narg('display_name')::text, display_name),
bio = COALESCE(sqlc.narg('bio')::text, bio),
location = COALESCE(sqlc.narg('location')::text, location)
-- We get the id from the middleware!!!
WHERE id = sqlc.arg('id')::uuid;

-- Temporarily inserting the user to 'pending_registrations' just to fetch the information later from it and register the user
-- name: FirstStepRegisterUser :exec
INSERT INTO pending_registrations(token, name, display_name, sex, birth_date, email, password_hash, created_at, expires_at)
VALUES (sqlc.arg('token')::text, sqlc.arg('name')::text, sqlc.arg('display_name')::text, sqlc.arg('sex')::text, sqlc.arg('birth_date')::date, sqlc.arg('email')::text, sqlc.arg('password_hash')::text, now(), now() + interval '5 minutes');

-- An SQL code to insert the users inside the database
-- name: SecondStepRegisterUser :one
INSERT INTO users (name, display_name, sex, birth_date, email, password_hash, created_at)
VALUES (sqlc.arg('name')::text, sqlc.arg('display_name')::text, sqlc.arg('sex'), sqlc.arg('birth_date')::timestamptz, sqlc.arg('email')::text, sqlc.arg('password_hash')::text, now())
RETURNING name, display_name, sex, birth_date, email, created_at;

-- Use id from context (middleware) to fetch info in login (as the password to check if it matches)
-- name: EmailForInfo :one
SELECT id, name, email, password_hash
FROM users
WHERE email = $1;

-- We use the token to fetch the info about the user
-- name: CheckToken :one
SELECT users.id, users.role 
FROM sessions 
JOIN users ON users.id = sessions.user_id 
WHERE sessions.token = $1 AND sessions.expires_at > now();

-- Simply insert the opaque token
-- name: CreateSession :exec
INSERT INTO sessions(token, created_at, expires_at)
VALUES (sqlc.arg('token'), now(), now() + interval '60 days');

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