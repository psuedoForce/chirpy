-- name: UpdateRefreshToken :one
UPDATE refresh_token
SET updated_at = $1, revoked_at = $1
WHERE token = $2
RETURNING *;