-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ? LIMIT 1;

-- name: CreateUser :exec
INSERT INTO users (username, password_hash) VALUES (?, ?);
