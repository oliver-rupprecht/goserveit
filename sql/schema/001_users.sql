-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY, 
    created_at TIMESTAMP NOT NULL, 
    updated_at TIMESTAMP NOT NULL, 
    email TEXT NOT NULL
);

ALTER TABLE users
ADD CONSTRAINT email_unique UNIQUE(email);

-- +goose Down
DROP TABLE users;
