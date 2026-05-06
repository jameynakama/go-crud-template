-- name: ListUsers :many
SELECT * FROM users
ORDER BY create_time DESC
LIMIT $1 OFFSET $2;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (username, is_admin)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET username = $2, is_admin = $3
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
