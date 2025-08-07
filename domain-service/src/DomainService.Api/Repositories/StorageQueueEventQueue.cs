using System.Text.Json;
using Azure.Storage.Queues;
using DomainService.Interfaces;

namespace DomainService.Repositories;

internal sealed class StorageQueueEventQueue : IEventQueue
{
    private readonly QueueClient _queue;
    public StorageQueueEventQueue(QueueClient queue) => _queue = queue;
    public Task Add(IEvent ev, CancellationToken ct)
        => _queue.SendMessageAsync(JsonSerializer.Serialize(ev), cancellationToken: ct);
}
