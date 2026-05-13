-- name: CreateDeliveryAttempt :one
INSERT INTO delivery_attempts (
    id,
    delivery_job_id,
    attempt_number,
    provider_name,
    status,
    request_payload,
    response_payload,
    error_message,
    duration_ms,
    attempted_at
) VALUES (
    @id,
    @delivery_job_id,
    @attempt_number,
    @provider_name,
    @status,
    @request_payload,
    @response_payload,
    @error_message,
    @duration_ms,
    @attempted_at
)
RETURNING *;

-- name: ListDeliveryAttemptsByJob :many
SELECT *
FROM delivery_attempts
WHERE delivery_job_id = @delivery_job_id
ORDER BY attempt_number ASC;
