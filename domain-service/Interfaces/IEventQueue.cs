namespace DomainService.Interfaces;

public interface IEventQueue
{
    Task Add(IEvent ev, CancellationToken ct);
}
