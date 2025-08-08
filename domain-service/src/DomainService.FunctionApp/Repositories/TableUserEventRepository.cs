using Azure.Data.Tables;
using DomainService.Interfaces;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class TableUserEventRepository(TableClient table) : IUserEventRepository
{
    private readonly TableClient _table = table;

    public async Task<bool> Exists(string userId, CancellationToken ct)
    {
        var filter = $"PartitionKey eq '{userId}'";
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
            {"UserId", ev.UserId},
            {"Data", JsonSerializer.Serialize(ev)}
        };
        await _table.AddEntityAsync(entity, ct);
    }
}
