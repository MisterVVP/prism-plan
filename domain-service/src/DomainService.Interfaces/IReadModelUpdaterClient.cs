namespace DomainService.Interfaces;

public interface IReadModelUpdaterClient
{
    Task SendAsync(IEvent ev, CancellationToken ct);
}
