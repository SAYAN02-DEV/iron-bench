-- name: CreateUser :one
INSERT INTO users (username, password, email)
VALUES ($1, $2, $3)
RETURNING id, username, password, email, created_at;

-- name: GetUserByID :one
SELECT id, username, password, email, created_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, username, password, email, created_at
FROM users
WHERE email = $1;

