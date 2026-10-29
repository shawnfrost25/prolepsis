-- name: InsertMockUser :one
INSERT INTO users(name, sex, birth_date, email, password_hash)
VALUES(
    sqlc.arg('name')::text,
    sqlc.arg('sex')::text,
    sqlc.arg('birth_date')::date,
    sqlc.arg('email')::text,
    sqlc.arg('password_hash')::text
) RETURNING id;