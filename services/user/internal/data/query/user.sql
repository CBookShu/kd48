-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ? LIMIT 1;
