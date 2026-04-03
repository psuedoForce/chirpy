-- +goose Up
CREATE TABLE refresh_token(
    token VARCHAR PRIMARY KEY UNIQUE NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    user_id UUID NOT NULL,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE refresh_token;