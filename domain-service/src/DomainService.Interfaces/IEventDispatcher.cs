namespace DomainService.Interfaces;

public interface IEventDispatcher
{
    Task Dispatch(IEvent ev, CancellationToken ct);
}
