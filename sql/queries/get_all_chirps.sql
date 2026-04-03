-- name: GetAllChirpsASC :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetAllChirpsDESC :many
SELECT * FROM chirps
ORDER BY created_at DESC;