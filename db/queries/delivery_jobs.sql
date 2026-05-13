-- name: CreateDeliveryJob :one
INSERT INTO delivery_jobs (
    id,
    notification_id,
    provider_id,
    channel,
    status,
    payload,
    send_at,
    max_retries,
    is_test
) VALUES (
    @id,
    @notification_id,
    @provider_id,
    @channel,
    @status,
    @payload,
    @send_at,
    @max_retries,
    @is_test
)
RETURNING *;

-- name: GetDeliveryJob :one
SELECT *
FROM delivery_jobs
WHERE id = @id;

-- name: ListDeliveryJobsByNotificationID :many
SELECT *
FROM delivery_jobs
WHERE notification_id = @notification_id
ORDER BY created_at DESC;

-- name: ListDeliveryJobs :many
SELECT *
FROM delivery_jobs
WHERE (@status = '' OR status = @status)
  AND (@channel = '' OR channel = @channel)
  AND (@notification_id = '' OR notification_id = @notification_id)
  AND send_at >= @from_time
  AND send_at <= @to_time
ORDER BY created_at DESC
LIMIT @limit_count
OFFSET @offset_count;

-- name: LockReadyJobs :many
WITH ready_jobs AS (
    SELECT id
    FROM delivery_jobs
    WHERE locked_at IS NULL
      AND (
        (status = 'pending' AND send_at <= NOW())
        OR (status = 'retrying' AND next_retry_at IS NOT NULL AND next_retry_at <= NOW())
      )
    ORDER BY send_at ASC, created_at ASC
    LIMIT @limit_count
    FOR UPDATE SKIP LOCKED
)
UPDATE delivery_jobs
SET status = 'processing',
    locked_at = NOW(),
    locked_by = @worker_id,
    heartbeat_at = NOW(),
    updated_at = NOW()
WHERE id IN (SELECT id FROM ready_jobs)
RETURNING *;

-- name: MarkJobSent :one
UPDATE delivery_jobs
SET status = 'sent',
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: MarkJobRetrying :one
UPDATE delivery_jobs
SET status = 'retrying',
    retry_count = retry_count + 1,
    next_retry_at = @next_retry_at,
    last_error = @last_error,
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: MarkJobDeadLetter :one
UPDATE delivery_jobs
SET status = 'dead_letter',
    last_error = @last_error,
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: CancelDeliveryJobsForNotification :many
UPDATE delivery_jobs
SET status = 'cancelled',
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = @updated_at
WHERE notification_id = @notification_id
  AND status IN ('pending', 'retrying')
RETURNING *;

-- name: RecoverStaleJobs :many
UPDATE delivery_jobs
SET status = CASE
        WHEN retry_count > 0 THEN 'retrying'
        ELSE 'pending'
    END,
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = @updated_at
WHERE locked_at IS NOT NULL
  AND heartbeat_at IS NOT NULL
  AND heartbeat_at < @stale_before
RETURNING *;

-- name: UpdateHeartbeat :execrows
UPDATE delivery_jobs
SET heartbeat_at = NOW(),
    updated_at = NOW()
WHERE locked_by = @worker_id
  AND locked_at IS NOT NULL;

-- name: RescheduleDeliveryJobsForNotification :execrows
UPDATE delivery_jobs
SET send_at = @send_at,
    updated_at = @updated_at
WHERE notification_id = @notification_id
  AND status IN ('pending', 'retrying');

-- name: UpdateDeliveryJobPayloadForNotification :execrows
UPDATE delivery_jobs
SET payload = @payload,
    updated_at = @updated_at
WHERE notification_id = @notification_id
  AND status = 'pending';
