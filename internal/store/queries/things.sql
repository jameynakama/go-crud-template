-- name: ListThings :many
SELECT * FROM things
ORDER BY update_time DESC
LIMIT $1 OFFSET $2;

-- name: GetThing :one
SELECT * FROM things WHERE id = $1;

-- name: CreateThing :one
INSERT INTO things (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateThing :one
UPDATE things
SET name = $2, description = $3
WHERE id = $1
RETURNING *;

-- name: DeleteThing :exec
DELETE FROM things WHERE id = $1;
