using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using Azure;
using Azure.Data.Tables;
using DomainService.Interfaces;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class TableTaskEventRepository(TableClient table) : ITaskEventRepository
{
    private const string IdempotencyPartitionKey = "__idempotency__";
    private const string StatusProperty = "Status";
    private const string InsertedAtProperty = "InsertedAt";
    private const string UpdatedAtProperty = "UpdatedAt";
    private const string ProcessingStatus = "Processing";
    private const string CompletedStatus = "Completed";
    private static readonly TimeSpan ProcessingLeaseDuration = TimeSpan.FromSeconds(30);

    private readonly TableClient _table = table;

    public async Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
    {
        var list = new List<IEvent>();
        var filter = $"PartitionKey eq '{EscapeFilterValue(taskId)}'";
        await foreach (var e in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (TryParseEvent(e, out Event? ev) && ev != null)
            {
                list.Add(ev);
            }
        }
        return [.. list.OrderBy(e => e.Timestamp)];
    }

    public async Task Add(IEvent ev, CancellationToken ct)
    {
        var insertedAt = DateTimeOffset.UtcNow;

        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"Type", ev.Type},
            {"EventTimestamp", ev.Timestamp},
            {"UserId", ev.UserId},
            {"IdempotencyKey", ev.IdempotencyKey},
            {"EntityType", ev.EntityType},
            {"Dispatched", false},
            {InsertedAtProperty, insertedAt},
        };

        if (ev.Data.HasValue)
        {
            entity.Add("Data", ev.Data.Value.GetRawText());
        }

        await _table.AddEntityAsync(entity, ct);
    }

    public async Task<bool> Exists(string idempotencyKey, CancellationToken ct)
    {
        var filter = $"IdempotencyKey eq '{EscapeFilterValue(idempotencyKey)}'";
        await foreach (var _ in _table.QueryAsync<TableEntity>(filter: filter, maxPerPage: 1, cancellationToken: ct))
        {
            return true;
        }
        return false;
    }

    public async Task<IReadOnlyList<StoredEvent>> FindByIdempotencyKey(string idempotencyKey, CancellationToken ct)
    {
        var filter = $"IdempotencyKey eq '{EscapeFilterValue(idempotencyKey)}'";
        var results = new List<(StoredEvent Stored, DateTimeOffset? InsertedAt, DateTimeOffset? TableTimestamp)>();
        await foreach (var entity in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (TryParseEvent(entity, out Event? ev) && ev != null)
            {
                var dispatched = entity.TryGetValue("Dispatched", out var dispatchedObj) && dispatchedObj is bool dispatchedFlag && dispatchedFlag;
                var insertedAt = ExtractDateTimeOffset(entity, InsertedAtProperty);
                results.Add((new StoredEvent(ev, dispatched), insertedAt, entity.Timestamp));
            }
        }
        results.Sort(static (left, right) =>
        {
            var timestampComparison = left.Stored.Event.Timestamp.CompareTo(right.Stored.Event.Timestamp);
            if (timestampComparison != 0)
            {
                return timestampComparison;
            }

            var insertedComparison = CompareInsertion(left.InsertedAt, right.InsertedAt);
            if (insertedComparison != 0)
            {
                return insertedComparison;
            }

            var tableTimestampComparison = Nullable.Compare(left.TableTimestamp, right.TableTimestamp);
            if (tableTimestampComparison != 0)
            {
                return tableTimestampComparison;
            }

            return string.CompareOrdinal(left.Stored.Event.Id, right.Stored.Event.Id);
        });

        return results.ConvertAll(static entry => entry.Stored);
    }

    private static int CompareInsertion(DateTimeOffset? left, DateTimeOffset? right)
    {
        return left.HasValue && right.HasValue
            ? left.Value.CompareTo(right.Value)
            : 0;
    }

    public Task MarkAsDispatched(IEvent ev, CancellationToken ct)
    {
        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"Dispatched", true},
        };

        return _table.UpdateEntityAsync(entity, ETag.All, TableUpdateMode.Merge, ct);
    }

    public async Task<IdempotencyResult> TryStartProcessing(string idempotencyKey, CancellationToken ct)
    {
        var entity = CreateIdempotencyEntity(idempotencyKey, ProcessingStatus);

        while (true)
        {
            try
            {
                await _table.AddEntityAsync(entity, ct);
                return IdempotencyResult.Started;
            }
            catch (RequestFailedException ex) when (AzTableHelpers.IsInsertConflict(ex))
            {
                var record = await GetIdempotencyRecord(idempotencyKey, ct);
                if (record is null)
                {
                    // The competing record disappeared between the failed insert and the lookup.
                    // Retry the loop so we can attempt the insert again.
                    continue;
                }

                if (string.Equals(record.Value.Status, CompletedStatus, StringComparison.Ordinal))
                {
                    return IdempotencyResult.AlreadyProcessed;
                }

                if (IsProcessingStale(record.Value.UpdatedAt) && await TryReclaimProcessing(record.Value.ETag, idempotencyKey, ct))
                {
                    return IdempotencyResult.Started;
                }

                return IdempotencyResult.InProgress;
            }
            catch (RequestFailedException ex)
            {
                Console.WriteLine($"Tables error status={ex.Status}, code={ex.ErrorCode}, msg={ex.Message}");
                throw;
            }
        }
    }

    public Task MarkProcessingSucceeded(string idempotencyKey, CancellationToken ct)
    {
        var entity = CreateIdempotencyEntity(idempotencyKey, CompletedStatus);
        return _table.UpsertEntityAsync(entity, TableUpdateMode.Merge, ct);
    }

    public async Task MarkProcessingFailed(string idempotencyKey, CancellationToken ct)
    {
        try
        {
            await _table.DeleteEntityAsync(IdempotencyPartitionKey, idempotencyKey, ETag.All, ct);
        }
        catch (RequestFailedException ex) when (ex.Status == 404)
        {
        }
    }

    private static TableEntity CreateIdempotencyEntity(string idempotencyKey, string status)
    {
        return new TableEntity(IdempotencyPartitionKey, idempotencyKey)
        {
            {StatusProperty, status},
            {UpdatedAtProperty, DateTimeOffset.UtcNow},
        };
    }

    private async Task<IdempotencyRecord?> GetIdempotencyRecord(string idempotencyKey, CancellationToken ct)
    {
        try
        {
            var response = await _table.GetEntityAsync<TableEntity>(IdempotencyPartitionKey, idempotencyKey, cancellationToken: ct);
            if (!response.HasValue)
            {
                return null;
            }

            var entity = response.Value;
            if (!entity.TryGetValue(StatusProperty, out var statusObj) || statusObj is not string status)
            {
                return null;
            }

            var updatedAt = ExtractDateTimeOffset(entity, UpdatedAtProperty);
            return new IdempotencyRecord(status, updatedAt, response.Value.ETag);
        }
        catch (RequestFailedException ex) when (ex.Status == 404)
        {
        }
        return null;
    }

    private static bool IsProcessingStale(DateTimeOffset? updatedAt)
    {
        if (!updatedAt.HasValue)
        {
            return true;
        }

        return DateTimeOffset.UtcNow - updatedAt.Value >= ProcessingLeaseDuration;
    }

    private async Task<bool> TryReclaimProcessing(ETag etag, string idempotencyKey, CancellationToken ct)
    {
        try
        {
            var takeover = CreateIdempotencyEntity(idempotencyKey, ProcessingStatus);
            await _table.UpdateEntityAsync(takeover, etag, TableUpdateMode.Replace, ct);
            return true;
        }
        catch (RequestFailedException ex) when (ex.Status == 404 || ex.Status == 412)
        {
            return false;
        }
    }

    private readonly record struct IdempotencyRecord(string Status, DateTimeOffset? UpdatedAt, ETag ETag);

    private static bool TryParseEvent(TableEntity entity, out Event? ev)
    {
        ev = null;

        if (!entity.TryGetValue("Type", out var typeObj) || typeObj is not string type)
        {
            return false;
        }

        var timestamp = ExtractInt64(entity, "EventTimestamp");
        var userId = entity.TryGetValue("UserId", out var userIdObj) && userIdObj is string uid ? uid : string.Empty;
        var idempotencyKey = entity.TryGetValue("IdempotencyKey", out var keyObj) && keyObj is string key ? key : string.Empty;
        var entityType = entity.TryGetValue("EntityType", out var entityTypeObj) && entityTypeObj is string et ? et : EntityTypes.Task;
        JsonElement? data = null;

        if (entity.TryGetValue("Data", out var dataObj) && dataObj is string dataText && !string.IsNullOrWhiteSpace(dataText) && dataText != "null")
        {
            using var doc = JsonDocument.Parse(dataText);
            data = doc.RootElement.Clone();
        }

        ev = new Event(entity.RowKey, entity.PartitionKey, entityType, type, data, timestamp, userId, idempotencyKey);
        return true;
    }

    private static long ExtractInt64(TableEntity entity, string key)
    {
        if (!entity.TryGetValue(key, out var value) || value is null)
        {
            return 0L;
        }

        return value switch
        {
            long l => l,
            int i => i,
            string s when long.TryParse(s, out var parsed) => parsed,
            DateTime dt => new DateTimeOffset(dt).ToUnixTimeMilliseconds(),
            DateTimeOffset dto => dto.ToUnixTimeMilliseconds(),
            _ => 0L,
        };
    }

    private static DateTimeOffset? ExtractDateTimeOffset(TableEntity entity, string key)
    {
        if (!entity.TryGetValue(key, out var value) || value is null)
        {
            return null;
        }

        return value switch
        {
            DateTimeOffset dto => dto,
            DateTime dt => new DateTimeOffset(DateTime.SpecifyKind(dt, DateTimeKind.Utc)),
            long l when l != 0 => DateTimeOffset.FromUnixTimeMilliseconds(l),
            string s when DateTimeOffset.TryParse(s, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind, out var parsed) => parsed,
            _ => null,
        };
    }

    private static string EscapeFilterValue(string value)
        => value.Replace("'", "''", StringComparison.Ordinal);
}
