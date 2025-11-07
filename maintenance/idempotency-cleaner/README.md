# Idempotency Cleaner Function

This Azure Functions project purges completed idempotency rows so the
Domain Service can reuse keys after commands finish. It runs on a timer
and deletes any entities under the `__idempotency__` partition whose
status is marked as `Completed`.

## Configuration

Set the following environment variables when deploying:

- `STORAGE_CONNECTION_STRING`: storage
  connection string.
- `IDEMPOTENCY_TABLES`: comma-separated list of tables to clean.
- `IDEMPOTENCY_CLEANER_SCHEDULE`: CRON expression describing how often
  to run the cleanup. Defaults to `0 */5 * * * *` (every five minutes).
