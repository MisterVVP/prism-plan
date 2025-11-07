import logging
import os
from typing import List

import azure.functions as func
from azure.core.exceptions import HttpResponseError, ResourceNotFoundError
from azure.data.tables import TableClient

IDEMPOTENCY_PARTITION_KEY = "__idempotency__"
STATUS_PROPERTY = "Status"
UPDATED_AT_PROPERTY = "UpdatedAt"
COMPLETED_STATUS = "Completed"


def main(timer: func.TimerRequest) -> None:
    logger = logging.getLogger("idempotency-cleaner")
    try:
        conn_str = os.getenv("STORAGE_CONNECTION_STRING")
        if not conn_str:
            logger.error("Idempotency cleanup skipped: no storage connection string configured")
            return

        table_names = _configured_tables()
        if not table_names:
            logger.warning("Idempotency cleanup skipped: no tables configured")
            return

        total_deleted = 0
        for table_name in table_names:
            try:
                table_client = TableClient.from_connection_string(conn_str, table_name)
            except Exception as exc:
                logger.error("Unable to create table client for %s: %s", table_name, exc)
                continue
            else:
                with table_client:
                    deleted = _cleanup_completed_entries(table_client, logger)
                    total_deleted += deleted

        logger.info("Idempotency cleanup completed: %d total rows removed", total_deleted)
    except Exception:
        logger.exception("Idempotency cleanup failed with an unexpected error")


def _configured_tables() -> List[str]:
    configured = os.getenv("IDEMPOTENCY_TABLES", "")
    if configured:
        return [name.strip() for name in configured.split(",") if name.strip()]
    else:
        raise Exception("IDEMPOTENCY_TABLES environment variable is invalid or not set!")

def _cleanup_completed_entries(client: TableClient, logger: logging.Logger) -> int:
    query_filter = (
        f"PartitionKey eq '{IDEMPOTENCY_PARTITION_KEY}' and {STATUS_PROPERTY} eq '{COMPLETED_STATUS}'"
    )
    select = ["PartitionKey", "RowKey", STATUS_PROPERTY, UPDATED_AT_PROPERTY]
    deleted = 0

    try:
        entities = client.query_entities(query_filter=query_filter, select=select)
    except ResourceNotFoundError:
        logger.warning("Table %s does not exist; skipping", client.table_name)
        return 0
    except HttpResponseError as exc:
        logger.error("Failed to query table %s: %s", client.table_name, exc)
        return 0

    try:
        for entity in entities:
            try:
                client.delete_entity(
                    partition_key=entity["PartitionKey"],
                    row_key=entity["RowKey"],
                )
                deleted += 1
            except ResourceNotFoundError:
                logger.warning(
                    "Completed idempotency row already removed from %s: %s",
                    client.table_name,
                    entity.get("RowKey"),
                )
            except HttpResponseError as exc:
                logger.error(
                    "Failed to delete completed idempotency row %s from %s: %s",
                    entity.get("RowKey"),
                    client.table_name,
                    exc,
                )
            except Exception:
                logger.exception(
                    "Unexpected error removing idempotency row %s from %s",
                    entity.get("RowKey"),
                    client.table_name,
                )
    except HttpResponseError as exc:
        logger.error("Failed while iterating completed rows in %s: %s", client.table_name, exc)

    return deleted
