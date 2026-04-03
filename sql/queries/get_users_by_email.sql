-- name: GetUsersByEmail :one
SELECT * FROM users
WHERE email = $1;
