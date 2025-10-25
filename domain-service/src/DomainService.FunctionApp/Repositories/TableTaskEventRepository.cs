using System;
using System.Collections.Generic;
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
    private const string UpdatedAtProperty = "UpdatedAt";
    private const string ProcessingStatus = "Processing";
    private const string CompletedStatus = "Completed";

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
        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"Type", ev.Type},
            {"Type@odata.type", "Edm.String"},
            {"EventTimestamp", ev.Timestamp},
            {"EventTimestamp@odata.type", "Edm.Int64"},
            {"UserId", ev.UserId},
            {"UserId@odata.type", "Edm.String"},
            {"IdempotencyKey", ev.IdempotencyKey},
            {"IdempotencyKey@odata.type", "Edm.String"},
            {"EntityType", ev.EntityType},
            {"EntityType@odata.type", "Edm.String"},
            {"Dispatched", false},
            {"Dispatched@odata.type", "Edm.Boolean"},
        };

        if (ev.Data.HasValue)
        {
            entity.Add("Data", ev.Data.Value.GetRawText());
            entity.Add("Data@odata.type", "Edm.String");
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
        var results = new List<(StoredEvent Stored, DateTimeOffset? InsertedAt)>();
        await foreach (var entity in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (TryParseEvent(entity, out Event? ev) && ev != null)
            {
                var dispatched = entity.TryGetValue("Dispatched", out var dispatchedObj) && dispatchedObj is bool dispatchedFlag && dispatchedFlag;
                results.Add((new StoredEvent(ev, dispatched), entity.Timestamp));
            }
        }
        results.Sort(static (left, right) =>
        {
            var timestampComparison = left.Stored.Event.Timestamp.CompareTo(right.Stored.Event.Timestamp);
            return timestampComparison != 0
                ? timestampComparison
                : OrderByInsertedThenId(left, right);
        });

        return results.ConvertAll(static entry => entry.Stored);
    }

    private static int OrderByInsertedThenId((StoredEvent Stored, DateTimeOffset? InsertedAt) left, (StoredEvent Stored, DateTimeOffset? InsertedAt) right)
    {
        var insertedComparison = Nullable.Compare(left.InsertedAt, right.InsertedAt);
        return insertedComparison != 0
            ? insertedComparison
            : string.CompareOrdinal(left.Stored.Event.Id, right.Stored.Event.Id);
    }

    public Task MarkAsDispatched(IEvent ev, CancellationToken ct)
    {
        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"Dispatched", true},
            {"Dispatched@odata.type", "Edm.Boolean"},
        };

        return _table.UpdateEntityAsync(entity, ETag.All, TableUpdateMode.Merge, ct);
    }

    public async Task<IdempotencyResult> TryStartProcessing(string idempotencyKey, CancellationToken ct)
    {
        var entity = CreateIdempotencyEntity(idempotencyKey, ProcessingStatus);
        try
        {
            await _table.AddEntityAsync(entity, ct);
            return IdempotencyResult.Started;
        }
        catch (RequestFailedException ex) when (ex.Status == 409)
        {
            var status = await GetIdempotencyStatus(idempotencyKey, ct);
            return string.Equals(status, CompletedStatus, StringComparison.Ordinal)
                ? IdempotencyResult.AlreadyProcessed
                : IdempotencyResult.InProgress;
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
            {StatusProperty + "@odata.type", "Edm.String"},
            {UpdatedAtProperty, DateTimeOffset.UtcNow},
            {UpdatedAtProperty + "@odata.type", "Edm.DateTimeOffset"},
        };
    }

    private async Task<string?> GetIdempotencyStatus(string idempotencyKey, CancellationToken ct)
    {
        try
        {
            var existing = await _table.GetEntityAsync<TableEntity>(IdempotencyPartitionKey, idempotencyKey, cancellationToken: ct);
            if (existing.HasValue && existing.Value.TryGetValue(StatusProperty, out var statusObj) && statusObj is string status)
            {
                return status;
            }
        }
        catch (RequestFailedException ex) when (ex.Status == 404)
        {
        }
        return null;
    }

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

    private static string EscapeFilterValue(string value)
        => value.Replace("'", "''", StringComparison.Ordinal);
}
