using Azure.Data.Tables;
using DomainService.Interfaces;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class TableTaskEventRepository(TableClient table) : ITaskEventRepository
{
    private readonly TableClient _table = table;

    public async Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
    {
        var list = new List<IEvent>();
        var filter = $"PartitionKey eq '{taskId}'";
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
            {"EventTimestamp", ev.Timestamp},
            {"UserId", ev.UserId},
            {"IdempotencyKey", ev.IdempotencyKey},
        };

        if (ev.Data.HasValue)
        {
            entity.Add("Data", ev.Data.Value.GetRawText());
        }

        await _table.AddEntityAsync(entity, ct);
    }

    public async Task<bool> Exists(string idempotencyKey, CancellationToken ct)
    {
        var filter = $"IdempotencyKey eq '{idempotencyKey}'";
        await foreach (var _ in _table.QueryAsync<TableEntity>(filter: filter, maxPerPage: 1, cancellationToken: ct))
        {
            return true;
        }
        return false;
    }

    private static bool TryParseEvent(TableEntity entity, out Event? ev)
    {
        ev = null;

        if (entity.TryGetValue("Type", out var typeObj))
        {
            if (typeObj is not string type)
            {
                return false;
            }

            var timestamp = ExtractInt64(entity, "EventTimestamp");
            var userId = entity.TryGetValue("UserId", out var userIdObj) && userIdObj is string uid ? uid : string.Empty;
            var idempotencyKey = entity.TryGetValue("IdempotencyKey", out var keyObj) && keyObj is string key ? key : string.Empty;
            JsonElement? data = null;

            if (entity.TryGetValue("Data", out var dataObj) && dataObj is string dataText && !string.IsNullOrWhiteSpace(dataText) && dataText != "null")
            {
                using var doc = JsonDocument.Parse(dataText);
                data = doc.RootElement.Clone();
            }

            ev = new Event(entity.RowKey, entity.PartitionKey, EntityTypes.Task, type, data, timestamp, userId, idempotencyKey);
            return true;
        }

        if (entity.TryGetValue("Data", out var legacyDataObj) && legacyDataObj is string legacyData)
        {
            ev = JsonSerializer.Deserialize<Event>(legacyData);
        }

        return ev != null;
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
}
