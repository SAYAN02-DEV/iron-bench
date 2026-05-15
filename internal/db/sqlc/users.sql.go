package sqlc

import (
	"context"
	"time"
)

type User struct {
	ID        int64
	Username  string
	Password  string
	Email     string
	CreatedAt time.Time
}

const createUser = `-- name: CreateUser :one
INSERT INTO users (username, password, email)
VALUES ($1, $2, $3)
RETURNING id, username, password, email, created_at`

func (q *Queries) CreateUser(ctx context.Context, username string, password string, email string) (User, error) {
	row := q.db.QueryRow(ctx, createUser, username, password, email)
	var i User
	err := row.Scan(&i.ID, &i.Username, &i.Password, &i.Email, &i.CreatedAt)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT id, username, password, email, created_at
FROM users
WHERE id = $1`

func (q *Queries) GetUserByID(ctx context.Context, id int64) (User, error) {
	row := q.db.QueryRow(ctx, getUserByID, id)
	var i User
	err := row.Scan(&i.ID, &i.Username, &i.Password, &i.Email, &i.CreatedAt)
	return i, err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, username, password, email, created_at
FROM users
WHERE email = $1`

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i User
	err := row.Scan(&i.ID, &i.Username, &i.Password, &i.Email, &i.CreatedAt)
	return i, err
}

