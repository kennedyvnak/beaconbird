-- name: CreateDeliveryJob :one
INSERT INTO delivery_jobs (
    id,
    notification_id,
    provider_id,
    channel,
    status,
    payload,
    send_at,
    max_retries
) VALUES (
    @id,
    @notification_id,
    @provider_id,
    @channel,
    @status,
    @payload,
    @send_at,
    @max_retries
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

-- name: ListReadyDeliveryJobs :many
SELECT *
FROM delivery_jobs
WHERE locked_at IS NULL
  AND (
    (status = 'pending' AND send_at <= @ready_at)
    OR (status = 'retrying' AND next_retry_at IS NOT NULL AND next_retry_at <= @ready_at)
  )
ORDER BY send_at ASC, created_at ASC
LIMIT @limit_count;

-- name: MarkDeliveryJobProcessing :one
UPDATE delivery_jobs
SET status = 'processing',
    locked_at = @locked_at,
    locked_by = @locked_by,
    heartbeat_at = @heartbeat_at,
    updated_at = @updated_at
WHERE id = @id
  AND locked_at IS NULL
RETURNING *;

-- name: MarkDeliveryJobSent :one
UPDATE delivery_jobs
SET status = 'sent',
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    last_error = NULL,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: MarkDeliveryJobForRetry :one
UPDATE delivery_jobs
SET status = 'retrying',
    retry_count = retry_count + 1,
    next_retry_at = @next_retry_at,
    last_error = @last_error,
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = @updated_at
WHERE id = @id
RETURNING *;

-- name: MarkDeliveryJobDeadLetter :one
UPDATE delivery_jobs
SET status = 'dead_letter',
    last_error = @last_error,
    locked_at = NULL,
    locked_by = NULL,
    heartbeat_at = NULL,
    updated_at = @updated_at
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

-- name: ResetStaleDeliveryJobs :many
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

-- name: UpdateHeartbeatForWorker :many
UPDATE delivery_jobs
SET heartbeat_at = @heartbeat_at,
    updated_at = @updated_at
WHERE locked_by = @locked_by
  AND status = 'processing'
RETURNING *;

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
