-- name: DeleteAllFromUsers :one
DELETE FROM users
RETURNING *;
