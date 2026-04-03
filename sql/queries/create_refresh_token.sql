-- name: InsertRefreshToken :exec
INSERT INTO refresh_token(
    token,
    created_at,
    updated_at,
    user_id,
    expires_at,
    revoked_at
)
VALUES($1, $2, $3, $4, $5, $6);