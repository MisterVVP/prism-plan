import logging
import os
from typing import Iterable, List

import azure.functions as func
from azure.core.exceptions import HttpResponseError, ResourceNotFoundError
from azure.data.tables import TableClient, TableTransactionAction, TableTransactionActionType

IDEMPOTENCY_PARTITION_KEY = "__idempotency__"
STATUS_PROPERTY = "Status"
UPDATED_AT_PROPERTY = "UpdatedAt"
COMPLETED_STATUS = "Completed"
MAX_BATCH_SIZE = 100


def main(timer: func.TimerRequest) -> None:  # pragma: no cover - entry point invoked by Azure Functions
    logger = logging.getLogger("idempotency-cleaner")
    conn_str = (
        os.getenv("IDEMPOTENCY_STORAGE_CONNECTION_STRING")
        or os.getenv("STORAGE_CONNECTION_STRING")
        or os.getenv("AzureWebJobsStorage")
    )
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
            client = TableClient.from_connection_string(conn_str, table_name)
        except Exception as exc:  # pragma: no cover - network/setup errors
            logger.error("Unable to create table client for %s: %s", table_name, exc)
            continue

        deleted = _cleanup_completed_entries(client, logger)
        total_deleted += deleted
        logger.info("Idempotency cleanup removed %d rows from %s", deleted, table_name)

    logger.info("Idempotency cleanup completed: %d total rows removed", total_deleted)


def _configured_tables() -> List[str]:
    configured = os.getenv("IDEMPOTENCY_TABLES", "")
    if configured:
        return [name.strip() for name in configured.split(",") if name.strip()]

    fallbacks = [
        os.getenv("TASK_EVENTS_TABLE"),
        os.getenv("USER_EVENTS_TABLE"),
    ]
    return [name for name in fallbacks if name]


def _cleanup_completed_entries(client: TableClient, logger: logging.Logger) -> int:
    filter_query = (
        f"PartitionKey eq '{IDEMPOTENCY_PARTITION_KEY}' and {STATUS_PROPERTY} eq '{COMPLETED_STATUS}'"
    )
    select = ["PartitionKey", "RowKey", STATUS_PROPERTY, UPDATED_AT_PROPERTY]
    deleted = 0
    batch: List[dict] = []

    try:
        entities = client.list_entities(results_per_page=None, filter=filter_query, select=select)
    except ResourceNotFoundError:
        logger.warning("Table %s does not exist; skipping", client.table_name)
        return 0
    except HttpResponseError as exc:  # pragma: no cover - transient errors
        logger.error("Failed to query table %s: %s", client.table_name, exc)
        return 0

    for entity in entities:
        batch.append(entity)
        if len(batch) >= MAX_BATCH_SIZE:
            deleted += _submit_deletes(client, batch, logger)
            batch.clear()

    if batch:
        deleted += _submit_deletes(client, batch, logger)

    return deleted


def _submit_deletes(client: TableClient, entities: Iterable[dict], logger: logging.Logger) -> int:
    actions = [
        TableTransactionAction(
            TableTransactionActionType.DELETE,
            {"PartitionKey": ent["PartitionKey"], "RowKey": ent["RowKey"]},
        )
        for ent in entities
    ]
    if not actions:
        return 0

    try:
        client.submit_transaction(actions)
        return len(actions)
    except ResourceNotFoundError:
        logger.warning(
            "One or more entities disappeared before deletion in table %s", client.table_name
        )
    except HttpResponseError as exc:  # pragma: no cover - transient errors
        logger.error("Failed to delete completed idempotency rows from %s: %s", client.table_name, exc)
    return 0
