-- The appointments module's queries. It writes only appointments and appointment_services; it
-- reads clients (id, name, archived_at) for client_name and the archived-client check.

-- name: GetClientState :one
SELECT archived_at FROM clients WHERE id = $1;

-- name: InsertAppointment :exec
INSERT INTO appointments (id, client_id, starts_at, ends_at, status, total_cents, attrs, notes,
                          created_at, updated_at)
VALUES (@id, @client_id, @starts_at, @ends_at, 'scheduled', @total_cents, @attrs::jsonb,
        sqlc.narg('notes')::text, @now::timestamptz, @now::timestamptz);

-- name: InsertAppointmentService :exec
INSERT INTO appointment_services (appointment_id, position, service_id, name, price_cents, duration_min)
-- duration_min is passed as bigint so the Go int needs no narrowing; the column CHECK and the
-- services rule (1 to 1440) keep it in range.
VALUES (@appointment_id, @position, @service_id, @name, sqlc.narg('price_cents')::bigint,
        @duration_min::bigint);

-- name: DeleteAppointmentServices :exec
DELETE FROM appointment_services WHERE appointment_id = @appointment_id;

-- name: UpdateAppointment :exec
UPDATE appointments SET
    starts_at   = @starts_at,
    ends_at     = @ends_at,
    total_cents = @total_cents,
    paid_cents  = sqlc.narg('paid_cents')::bigint,
    attrs       = @attrs::jsonb,
    notes       = sqlc.narg('notes')::text,
    updated_at  = @now::timestamptz
WHERE id = @id;

-- name: UpdateAppointmentStatus :exec
UPDATE appointments SET status = @status, updated_at = @now::timestamptz WHERE id = @id;

-- name: GetAppointment :one
SELECT a.id, a.client_id, c.name AS client_name, a.starts_at, a.ends_at, a.status, a.total_cents,
       a.paid_cents, a.attrs, a.notes, a.created_at, a.updated_at
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.id = @id;

-- name: ListAppointmentsInRange :many
SELECT a.id, a.client_id, c.name AS client_name, a.starts_at, a.ends_at, a.status, a.total_cents,
       a.paid_cents, a.attrs, a.notes, a.created_at, a.updated_at
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.starts_at >= @range_start::timestamptz AND a.starts_at < @range_end::timestamptz
ORDER BY a.starts_at, a.id;

-- name: ListAppointmentsForClient :many
SELECT a.id, a.client_id, c.name AS client_name, a.starts_at, a.ends_at, a.status, a.total_cents,
       a.paid_cents, a.attrs, a.notes, a.created_at, a.updated_at
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.client_id = @client_id
ORDER BY a.starts_at DESC, a.id DESC;

-- name: ListOverlapping :many
SELECT a.id, a.client_id, c.name AS client_name, a.starts_at, a.ends_at, a.status, a.total_cents,
       a.paid_cents, a.attrs, a.notes, a.created_at, a.updated_at
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.status IN ('scheduled','confirmed') AND a.starts_at < @ends_at AND a.ends_at > @starts_at AND a.id <> @exclude
ORDER BY a.starts_at, a.id;

-- name: ListAppointmentServices :many
SELECT appointment_id, position, service_id, name, price_cents, duration_min
FROM appointment_services
WHERE appointment_id = ANY(@ids::uuid[])
ORDER BY appointment_id, position;
