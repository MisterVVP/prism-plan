using Azure.Data.Tables;
using DomainService.Interfaces;
using System.Globalization;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class TableTaskEventRepository(TableClient table) : ITaskEventRepository
{
    private static readonly string[] StatusColumns = new[] { "Type", "EventTimestamp", "Data" };
    private readonly TableClient _table = table;

    public async Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
    {
        var list = new List<IEvent>();
        var filter = $"PartitionKey eq '{taskId}'";
        await foreach (var e in _table.QueryAsync<TableEntity>(filter: filter, cancellationToken: ct))
        {
            if (e.TryGetValue("Data", out object? dataObj) && dataObj is string data)
            {
                var ev = JsonSerializer.Deserialize<Event>(data);
                if (ev != null) list.Add(ev);
            }
        }
        return [.. list.OrderBy(e => e.Timestamp)];
    }

    public async Task<TaskEventStatus> GetStatus(string taskId, CancellationToken ct)
    {
        var filter = $"PartitionKey eq '{taskId}'";
        var exists = false;
        var hasStatusEvent = false;
        var latestTimestamp = long.MinValue;
        var done = false;

        await foreach (var entity in _table.QueryAsync<TableEntity>(filter: filter, select: StatusColumns, cancellationToken: ct))
        {
            exists = true;
            if (!TryReadEventMetadata(entity, out var eventType, out var timestamp))
            {
                continue;
            }

            if (eventType != TaskEventTypes.Completed && eventType != TaskEventTypes.Reopened)
            {
                continue;
            }

            if (!hasStatusEvent || timestamp > latestTimestamp)
            {
                latestTimestamp = timestamp;
                done = eventType == TaskEventTypes.Completed;
                hasStatusEvent = true;
            }
        }

        return new TaskEventStatus(exists, hasStatusEvent && done);
    }

    public async Task Add(IEvent ev, CancellationToken ct)
    {
        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"UserId", ev.UserId},
            {"Type", ev.Type},
            {"EventTimestamp", ev.Timestamp},
            {"Data", JsonSerializer.Serialize(ev)},
            {"IdempotencyKey", ev.IdempotencyKey}
        };
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

    private static bool TryReadEventMetadata(TableEntity entity, out string eventType, out long timestamp)
    {
        eventType = string.Empty;
        timestamp = default;

        string? resolvedType = null;
        long? resolvedTimestamp = null;

        if (entity.TryGetValue("Type", out var typeValue) && typeValue is string type && !string.IsNullOrEmpty(type))
        {
            resolvedType = type;
        }

        if (entity.TryGetValue("EventTimestamp", out var timestampValue) && TryConvertTimestamp(timestampValue, out var ts))
        {
            resolvedTimestamp = ts;
        }

        if (resolvedType is not null && resolvedTimestamp is not null)
        {
            eventType = resolvedType;
            timestamp = resolvedTimestamp.Value;
            return true;
        }

        if (!entity.TryGetValue("Data", out var dataValue) || dataValue is not string payload || string.IsNullOrEmpty(payload))
        {
            if (resolvedType is not null && resolvedTimestamp is not null)
            {
                eventType = resolvedType;
                timestamp = resolvedTimestamp.Value;
                return true;
            }

            return false;
        }

        using var document = JsonDocument.Parse(payload);
        var root = document.RootElement;

        if (resolvedType is null && root.TryGetProperty(nameof(Event.Type), out var typeProperty) && typeProperty.ValueKind == JsonValueKind.String)
        {
            var parsedType = typeProperty.GetString();
            if (!string.IsNullOrEmpty(parsedType))
            {
                resolvedType = parsedType;
            }
        }

        if (resolvedTimestamp is null && root.TryGetProperty(nameof(Event.Timestamp), out var timestampProperty) && timestampProperty.TryGetInt64(out var parsedTimestamp))
        {
            resolvedTimestamp = parsedTimestamp;
        }

        if (resolvedType is not null && resolvedTimestamp is not null)
        {
            eventType = resolvedType;
            timestamp = resolvedTimestamp.Value;
            return true;
        }

        return false;
    }

    private static bool TryConvertTimestamp(object value, out long timestamp)
    {
        switch (value)
        {
            case long longValue:
                timestamp = longValue;
                return true;
            case int intValue:
                timestamp = intValue;
                return true;
            case string stringValue when long.TryParse(stringValue, NumberStyles.Integer, CultureInfo.InvariantCulture, out var parsed):
                timestamp = parsed;
                return true;
            default:
                timestamp = default;
                return false;
        }
    }
}
