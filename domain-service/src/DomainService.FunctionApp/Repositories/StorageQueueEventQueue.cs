using Azure.Storage.Queues;
using DomainService.Interfaces;
using System.Text.Json;

namespace DomainService.Repositories;

internal sealed class StorageQueueEventQueue(QueueClient queue) : IEventQueue
{
    private readonly QueueClient _queue = queue;

    public Task Add(IEvent ev, CancellationToken ct)
        => _queue.SendMessageAsync(JsonSerializer.Serialize(ev), cancellationToken: ct);
}
