# Idempotency Cleaner Function

This Azure Functions project purges completed idempotency rows so the
Domain Service can reuse keys after commands finish. It runs on a timer
and deletes any entities under the `__idempotency__` partition whose
status is marked as `Completed`.

## Configuration

Set the following environment variables when deploying:

- `IDEMPOTENCY_STORAGE_CONNECTION_STRING` (optional): overrides the
  storage account connection string.
- `STORAGE_CONNECTION_STRING` or `AzureWebJobsStorage`: fallback storage
  connection strings.
- `IDEMPOTENCY_TABLES`: comma-separated list of tables to clean. When
  not provided the function falls back to `TASK_EVENTS_TABLE` and
  `USER_EVENTS_TABLE` if present.
- `IDEMPOTENCY_CLEANER_SCHEDULE`: CRON expression describing how often
  to run the cleanup. Defaults to `0 */5 * * * *` (every five minutes).

The function uses batch deletes (up to 100 entities per request) and
ignores tables that are missing or already empty.
