-- name: GetuserFromRefreshToken :one
SELECT * FROM users
INNER JOIN refresh_token
ON refresh_token.user_id = users.id
WHERE refresh_token.token = $1;
