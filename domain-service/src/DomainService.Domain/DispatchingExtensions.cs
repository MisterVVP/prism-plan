using System;
using System.Linq;
using DomainService.Interfaces;

namespace DomainService.Domain;

internal static class DispatchingExtensions
{
    public static async Task<bool> ReplayStoredEvents(this IDispatchAwareEventRepository repository, IEventDispatcher dispatcher, string idempotencyKey, CancellationToken ct)
    {
        var storedEvents = await repository.FindByIdempotencyKey(idempotencyKey, ct);
        if (storedEvents.Count == 0)
        {
            return false;
        }

        foreach (var stored in storedEvents
            .OrderBy(static e => e.Event.Timestamp)
            .ThenBy(static e => e.StoredAt)
            .ThenBy(static e => e.Event.Id, StringComparer.Ordinal))
        {
            if (!stored.Dispatched)
            {
                await dispatcher.Dispatch(stored.Event, ct);
                await repository.MarkAsDispatched(stored.Event, ct);
            }
        }

        return true;
    }
}
