namespace DomainService.Interfaces;

public interface IDispatchAwareEventRepository
{
    Task Add(IEvent ev, CancellationToken ct);
    Task<IReadOnlyList<StoredEvent>> FindByIdempotencyKey(string idempotencyKey, CancellationToken ct);
    Task MarkAsDispatched(IEvent ev, CancellationToken ct);
}

public interface ITaskEventRepository : IDispatchAwareEventRepository
{
    Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct);
    Task<bool> Exists(string idempotencyKey, CancellationToken ct);
}
