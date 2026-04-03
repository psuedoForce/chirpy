-- +goose Up
CREATE TABLE users(
    id UUID PRIMARY KEY NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    email VARCHAR
);

-- +goose Down
DROP TABLE users;