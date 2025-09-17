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
            {"Type", ev.Type},
            {"Type@odata.type", "Edm.String"},
            {"EventTimestamp", ev.Timestamp},
            {"EventTimestamp@odata.type", "Edm.Int64"},
            {"UserId", ev.UserId},
            {"UserId@odata.type", "Edm.String"},
            {"IdempotencyKey", ev.IdempotencyKey},
            {"IdempotencyKey@odata.type", "Edm.String"},
        };

        if (ev.Data.HasValue)
        {
            entity.Add("Data", ev.Data.Value.GetRawText());
            entity.Add("Data@odata.type", "Edm.String");
        }

        await _table.AddEntityAsync(entity, ct);
    }
}
