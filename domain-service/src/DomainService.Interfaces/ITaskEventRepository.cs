namespace DomainService.Interfaces;

public interface ITaskEventRepository
{
    Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct);
    Task Add(IEvent ev, CancellationToken ct);
    Task<bool> Exists(string idempotencyKey, CancellationToken ct);
}
