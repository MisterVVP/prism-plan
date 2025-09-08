using Azure.Data.Tables;
using DomainService.Interfaces;
using System;
using System.Text.Json;
using System.Linq;

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
            if (e.TryGetValue("Data", out object? dataObj) && dataObj is string data)
            {
                var ev = JsonSerializer.Deserialize<Event>(data);
                if (ev != null) list.Add(ev);
            }
        }
        return list
            .OrderBy(e => e.Timestamp)
            .ThenBy(e => e.Id, StringComparer.Ordinal)
            .ToList();
    }

    public async Task Add(IEvent ev, CancellationToken ct)
    {
        var entity = new TableEntity(ev.EntityId, ev.Id)
        {
            {"UserId", ev.UserId},
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
}
