-- Temporarily inserting the user to 'pending_registrations' just to fetch the information later from it and register the user
-- name: FirstStepRegisterUser :exec
INSERT INTO pending_registrations(token, name, sex, birth_date, email, password_hash, created_at, expires_at)
VALUES (sqlc.arg('token')::text, sqlc.arg('name')::text, sqlc.arg('sex')::text, sqlc.arg('birth_date')::date, sqlc.arg('email')::text, sqlc.arg('password_hash')::text, now(), now() + interval '3 minutes');

-- We fetch the info, so we can insert it later inside "users"
-- name: FetchPendingRegistrationsInfo :one
SELECT token, name, sex, birth_date, email, password_hash, created_at, expires_at
FROM pending_registrations
WHERE token = $1 AND expires_at > now();

-- We delete everything, cuz yes
-- name: DeleteRegistrationSteps :exec
DELETE FROM pending_registrations
WHERE email = $1;

-- We delete everything if the given status is 'created' or the token is 'expired'
-- name: DeleteWhereDone :exec
DELETE FROM pending_registrations
WHERE status = 'created' OR now() > expires_at;

-- We update the status and fluff after creation
-- name: UpdatePendingStatus :exec
UPDATE pending_registrations
SET status  = 'created'
WHERE token = $1;

-- We check if the user exists inside pending_registrations, and if the token is even alright at this point
-- name: ExistsInPendingRegistrations :one
SELECT EXISTS (
    SELECT 1
    FROM pending_registrations
    WHERE email = $1 AND expires_at > now()
);
