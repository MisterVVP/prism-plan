namespace DomainService.Interfaces;

public interface IUserEventRepository
{
    Task<bool> Exists(string userId, CancellationToken ct);
    Task Add(IEvent ev, CancellationToken ct);
}
