-- name: CreateNotification :one
INSERT INTO notifications (
    id,
    idempotency_key,
    status,
    tag,
    content,
    metadata,
    send_at
) VALUES (
    @id,
    @idempotency_key,
    @status,
    @tag,
    @content,
    @metadata,
    @send_at
)
RETURNING *;

-- name: GetNotification :one
SELECT *
FROM notifications
WHERE id = @id;

-- name: GetNotificationByIdempotencyKey :one
SELECT *
FROM notifications
WHERE idempotency_key = @idempotency_key;

-- name: ListNotifications :many
SELECT *
FROM notifications
WHERE (@status = '' OR status = @status)
  AND (@tag = '' OR tag = @tag)
  AND send_at >= @from_time
  AND send_at <= @to_time
ORDER BY created_at DESC
LIMIT @limit_count
OFFSET @offset_count;

-- name: UpdateNotificationStatus :one
UPDATE notifications
SET status = @status,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: CancelNotification :one
UPDATE notifications
SET status = 'cancelled',
    cancelled_at = @cancelled_at,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: RescheduleNotification :one
UPDATE notifications
SET send_at = @send_at,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: UpdateNotificationContent :one
UPDATE notifications
SET content = @content,
    metadata = @metadata,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;
