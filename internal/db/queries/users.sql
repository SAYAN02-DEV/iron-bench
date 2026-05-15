-- name: CreateUser :one
INSERT INTO users (username, email, password)
VALUES ($1, $2, $3)
RETURNING id, username, email;

-- name: GetUserByEmail :one
SELECT id, username, email, password
FROM users
WHERE email = $1;
