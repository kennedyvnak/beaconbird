-- name: CreateAPIKey :one
INSERT INTO api_keys (
    id,
    name,
    key_hash,
    mode
) VALUES (
    @id,
    @name,
    @key_hash,
    @mode
)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT *
FROM api_keys
WHERE key_hash = @key_hash
  AND is_revoked = FALSE;

-- name: ListAPIKeys :many
SELECT *
FROM api_keys
ORDER BY created_at DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET is_revoked = TRUE,
    revoked_at = @revoked_at
WHERE id = @id
RETURNING *;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = @last_used_at
WHERE id = @id;
