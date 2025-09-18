using Azure;
using Azure.Data.Tables;
using DomainService.Interfaces;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class TableUserEventRepository(TableClient table) : IUserEventRepository
{
    private readonly TableClient _table = table;

    public async Task<bool> Exists(string userId, CancellationToken ct)
    {
        var filter = $"PartitionKey eq '{EscapeFilterValue(userId)}'";
        await foreach (var _ in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            return true;
        }
        return false;
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

    public async Task<IReadOnlyList<StoredEvent>> FindByIdempotencyKey(string idempotencyKey, CancellationToken ct)
    {
        var filter = $"IdempotencyKey eq '{EscapeFilterValue(idempotencyKey)}'";
        var results = new List<StoredEvent>();
        await foreach (var entity in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (TryParseEvent(entity, out Event? ev) && ev != null)
            {
                var dispatched = entity.TryGetValue("Dispatched", out var dispatchedObj) && dispatchedObj is bool dispatchedFlag && dispatchedFlag;
                results.Add(new StoredEvent(ev, dispatched));
            }
        }
        return results;
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
        var entityType = entity.TryGetValue("EntityType", out var entityTypeObj) && entityTypeObj is string et ? et : EntityTypes.User;
        System.Text.Json.JsonElement? data = null;

        if (entity.TryGetValue("Data", out var dataObj) && dataObj is string dataText && !string.IsNullOrWhiteSpace(dataText) && dataText != "null")
        {
            using var doc = System.Text.Json.JsonDocument.Parse(dataText);
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
