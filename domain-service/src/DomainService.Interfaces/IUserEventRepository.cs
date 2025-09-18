namespace DomainService.Interfaces;

public interface IUserEventRepository : IDispatchAwareEventRepository
{
    Task<bool> Exists(string userId, CancellationToken ct);
}
