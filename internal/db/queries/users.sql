-- An SQL code to fetch user information based on the given ID
-- name: GetUserByID :one
SELECT name, display_name, bio, sex, location, birth_date, email, role, created_at
FROM users
WHERE id = $1;

-- An SQL code to add info insdie the not-forced fields (display_name, bio, location)
-- name: UpdateUserNotForced :exec
UPDATE users
SET 
display_name = COALESCE(sqlc.narg('display_name')::text, display_name),
bio = COALESCE(sqlc.narg('bio')::text, bio),
location = COALESCE(sqlc.narg('location')::text, location)
-- We get the id from the middleware!!!
WHERE id = sqlc.arg('id')::uuid;

-- We also check if the user exists in the database with the same email before registering
-- name: UserExists :one
SELECT EXISTS (
    SELECT 1
    FROM users
    WHERE email = $1 
);

-- An SQL code to insert the users inside the database
-- name: SecondStepRegisterUser :one
INSERT INTO users (name, display_name, sex, birth_date, email, password_hash, created_at)
VALUES (sqlc.arg('name')::text, sqlc.arg('display_name')::text, sqlc.arg('sex'), sqlc.arg('birth_date')::date, sqlc.arg('email')::text, sqlc.arg('password_hash')::text, now())
RETURNING id, name, display_name, sex, birth_date, email, created_at;

-- We simply get info for later (inside login) - so we can update the session
-- name: EmailForInfo :one
SELECT id, name, email, password_hash
FROM users
WHERE email = $1;