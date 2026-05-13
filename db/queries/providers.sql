-- name: CreateProvider :one
INSERT INTO providers (
    id,
    name,
    channel,
    config,
    is_active
) VALUES (
    @id,
    @name,
    @channel,
    @config,
    @is_active
)
RETURNING *;

-- name: GetProvider :one
SELECT *
FROM providers
WHERE id = @id;

-- name: ListProvidersByChannel :many
SELECT *
FROM providers
WHERE channel = @channel
  AND is_active = TRUE
ORDER BY name ASC;

-- name: UpdateProviderConfig :one
UPDATE providers
SET config = @config,
    is_active = @is_active,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;
