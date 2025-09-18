using DomainService.Interfaces;
using Microsoft.Extensions.Logging;

namespace DomainService.Domain;

public sealed class ResilientEventDispatcher(IEventQueue queue, IReadModelUpdaterClient fallbackClient, ILogger<ResilientEventDispatcher> logger) : IEventDispatcher
{
    private readonly IEventQueue _queue = queue;
    private readonly IReadModelUpdaterClient _fallbackClient = fallbackClient;
    private readonly ILogger<ResilientEventDispatcher> _logger = logger;

    public async Task Dispatch(IEvent ev, CancellationToken ct)
    {
        try
        {
            await _queue.Add(ev, ct);
            return;
        }
        catch (OperationCanceledException)
        {
            throw;
        }
        catch (Exception ex) when (!ct.IsCancellationRequested)
        {
            _logger.LogWarning(ex, "Queue dispatch failed for event {EventId}, falling back to HTTP", ev.Id);
        }

        await _fallbackClient.SendAsync(ev, ct);
    }
}
